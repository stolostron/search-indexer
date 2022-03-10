// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/driftprogramming/pgxpoolmock"
	pgxpool "github.com/jackc/pgx/v4/pgxpool"
	"github.com/stolostron/search-indexer/pkg/config"
	"github.com/stolostron/search-indexer/pkg/model"
	"k8s.io/klog/v2"
)

var ExistingClustersMap map[string]interface{} // a map to hold Current clusters and properties

// Database Access Object. Use a DAO instance so we can replace the pool object in the unit tests.
type DAO struct {
	pool      pgxpoolmock.PgxPool
	batchSize int
}

var poolSingleton pgxpoolmock.PgxPool

// Creates new DAO instance.
func NewDAO(p pgxpoolmock.PgxPool) DAO {
	// Crete DAO with default values.
	dao := DAO{
		batchSize: 500,
	}
	if p != nil {
		dao.pool = p
		return dao
	}

	if poolSingleton == nil {
		poolSingleton = initializePool()
	}
	dao.pool = poolSingleton
	return dao
}

func initializePool() pgxpoolmock.PgxPool {
	cfg := config.Cfg

	databaseUrl := fmt.Sprintf("postgresql://%s:%s@%s:%d/%s", cfg.DBUser, cfg.DBPass, cfg.DBHost, cfg.DBPort, cfg.DBName)
	klog.Infof("Connecting to PostgreSQL at: postgresql://%s:%s@%s:%d/%s",
		cfg.DBUser, "*****", cfg.DBHost, cfg.DBPort, cfg.DBName)

	config, configErr := pgxpool.ParseConfig(databaseUrl)
	if configErr != nil {
		klog.Fatal("Error parsing database connection configuration. ", configErr)
	}

	conn, err := pgxpool.ConnectConfig(context.Background(), config)
	if err != nil {
		klog.Error("Unable to connect to database: %+v\n", err)
		// TODO: We need to retry the connection until successful.
	} else {
		klog.Info("Successfully connected to database!")
	}

	return conn
}

func (dao *DAO) InitializeTables() {
	ExistingClustersMap = make(map[string]interface{})
	if config.Cfg.DevelopmentMode {
		klog.Warning("Dropping search schema for development only. We must not see this message in production.")
		_, err := dao.pool.Exec(context.Background(), "DROP SCHEMA IF EXISTS search CASCADE")
		checkError(err, "Error dropping schema search.")
	}

	_, err := dao.pool.Exec(context.Background(), "CREATE SCHEMA IF NOT EXISTS search")
	checkError(err, "Error creating schema.")
	_, err = dao.pool.Exec(context.Background(),
		"CREATE TABLE IF NOT EXISTS search.resources (uid TEXT PRIMARY KEY, cluster TEXT, data JSONB)")
	checkError(err, "Error creating table search.resources.")
	_, err = dao.pool.Exec(context.Background(),
		"CREATE TABLE IF NOT EXISTS search.edges (sourceId TEXT, sourceKind TEXT,destId TEXT,destKind TEXT,edgeType TEXT,cluster TEXT, PRIMARY KEY(sourceId, destId, edgeType))")
	checkError(err, "Error creating table search.edges.")

	// Jsonb indexing data keys:
	_, err = dao.pool.Exec(context.Background(),
		"CREATE INDEX IF NOT EXISTS data_kind_idx ON search.resources USING GIN ((data -> 'kind'))")
	checkError(err, "Error creating index on search.resources data key kind.")

	_, err = dao.pool.Exec(context.Background(),
		"CREATE INDEX IF NOT EXISTS data_namespace_idx ON search.resources USING GIN ((data -> 'namespace'))")
	checkError(err, "Error creating index on search.resources data key namespace.")

	_, err = dao.pool.Exec(context.Background(),
		"CREATE INDEX IF NOT EXISTS data_name_idx ON search.resources USING GIN ((data ->  'name'))")
	checkError(err, "Error creating index on search.resources data key name.")

}

func checkError(err error, logMessage string) {
	if err != nil {
		klog.Error(logMessage, " ", err)
	}
}

func (dao *DAO) UpsertCluster(resource model.Resource) {
	data, _ := json.Marshal(resource.Properties)
	var query string
	var args []interface{}
	clusterName := resource.Properties["name"].(string)

	// Insert cluster node if cluster does not exist in the DB
	if !dao.ClusterInDB(resource.Properties["name"].(string)) {
		args = []interface{}{resource.UID, "", string(data)}
		klog.Infof("Cluster %s does not exist in DB, inserting it.", clusterName)
		query = "INSERT into search.resources values($1,$2,$3)"
	} else {
		// Check if the cluster properties are up to date in the DB
		if !dao.ClusterPropsUpToDate(clusterName, resource) {
			args = []interface{}{resource.UID, string(data)}
			klog.V(3).Infof("Cluster %s already exists in DB. Updating properties.", clusterName)
			query = "UPDATE search.resources SET data=$2 WHERE uid=$1"
		} else {
			klog.V(3).Infof("Cluster %s already exists in DB and properties are up to date.", clusterName)
			return
		}
	}
	_, err := dao.pool.Exec(context.TODO(), query, args...)
	if err != nil {
		klog.Warningf("Error inserting/updating cluster %s: %s", clusterName, err.Error())
	} else {
		ExistingClustersMap[resource.UID] = resource.Properties
	}
}

func (dao *DAO) ClusterInDB(clusterName string) bool {
	clusterUID := string("cluster__" + clusterName)
	_, ok := ExistingClustersMap[clusterUID]
	if !ok {
		klog.Infof("cluster %s not in ExistingClustersMap. Checking in db", clusterName)
		query := "SELECT uid, data from search.resources where uid=$1"
		rows, err := dao.pool.Query(context.TODO(), query, clusterUID)
		if err != nil {
			klog.Errorf("Error while checking if cluster already exists in DB: %s", err.Error())
		}
		for rows.Next() {
			var uid string
			var data interface{}
			err := rows.Scan(&uid, &data)
			if err != nil {
				klog.Errorf("Error %s retrieving rows for query:%s", err.Error(), query)
			} else {
				ExistingClustersMap[uid] = data
			}
		}
		_, ok = ExistingClustersMap[clusterUID]
	}
	return ok
}

func (dao *DAO) ClusterPropsUpToDate(clusterName string, resource model.Resource) bool {
	clusterUID := string("cluster__" + clusterName)
	currProps := resource.Properties
	existingProps, ok := ExistingClustersMap[clusterUID].(map[string]interface{})
	if ok && len(existingProps) == len(currProps) {
		return reflect.DeepEqual(currProps, existingProps)
	} else {
		klog.Infof("For cluster %s, properties needs to be updated.", clusterName)
		return false
	}
}
