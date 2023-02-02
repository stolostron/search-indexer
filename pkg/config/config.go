// Copyright Contributors to the Open Cluster Management project

package config

import (
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strconv"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const COMPONENT_VERSION = "2.7.0"

var DEVELOPMENT_MODE = false // Do not change this. See config_development.go to enable.
var Cfg = new()

// Struct to hold our configuratioin
type Config struct {
	DBBatchSize    int // Batch size used to write to DB.
	DBHost         string
	DBMaxConns     int // Max size of DB connection pool. Default: 30
	DBPort         int
	DBName         string
	DBUser         string
	DBPass         string
	HTTPTimeout    int // timeout when the http server should drop connections
	KubeConfigPath string
	KubeClient     *kubernetes.Clientset
	PodName        string
	PodNamespace   string
	ServerAddress  string // Web server address
	SlowLog        int    // Log operations slower than the specified time in ms. Default 1000ms
	Version        string
	MaxBackoffMS   int // Maximum backoff in ms to wait after db connection error
	// EdgeBuildRateMS       int    // rate at which intercluster edges should be build
	RediscoverRateMS int // time in MS we should check on cluster resource type
	RequestLimit     int // Max number of concurrent requests. Used to prevent from overloading the database
	// SkipClusterValidation string // Skips cluster validation. Intended only for performance tests.
	DevelopmentMode bool
}

// Reads config from environment.
func new() *Config {
	conf := &Config{
		DevelopmentMode: DEVELOPMENT_MODE, // Do not read this from ENV. See config_development.go to enable.
		DBBatchSize:     getEnvAsInt("DB_BATCH_SIZE", 500),
		DBHost:          getEnv("DB_HOST", "localhost"),
		DBMaxConns:      getEnvAsInt("DB_MAX_CONNS", int(30)),
		DBPort:          getEnvAsInt("DB_PORT", 5432),
		DBName:          getEnv("DB_NAME", ""),
		DBUser:          getEnv("DB_USER", ""),
		DBPass:          getEnv("DB_PASS", ""),
		HTTPTimeout:     getEnvAsInt("HTTP_TIMEOUT", 300000), // 5 min
		KubeConfigPath:  getKubeConfigPath(),
		PodName:         getEnv("POD_NAME", "local-dev"),
		PodNamespace:    getEnv("POD_NAMESPACE", "open-cluster-management"),
		ServerAddress:   getEnv("AGGREGATOR_ADDRESS", ":3010"),
		SlowLog:         getEnvAsInt("SLOW_LOG", int(1000)), // 1 second
		Version:         COMPONENT_VERSION,
		// Use 5 min for delete cluster activities and 30 seconds for db reconnect retry
		MaxBackoffMS: getEnvAsInt("MAX_BACKOFF_MS", 300000), // 5 min
		// EdgeBuildRateMS:       getEnvAsInt("EDGE_BUILD_RATE_MS", 15000), // 15 sec
		RediscoverRateMS: getEnvAsInt("REDISCOVER_RATE_MS", 300000), // 5 min
		RequestLimit:     getEnvAsInt("REQUEST_LIMIT", 50),
		// SkipClusterValidation: getEnvAsBool("SKIP_CLUSTER_VALIDATION", false),
	}

	// URLEncode the db password.
	conf.DBPass = url.QueryEscape(conf.DBPass)

	// Initialize Kube Client
	conf.KubeClient = getKubeClient()

	return conf
}

// Format and print environment to logger.
func (cfg *Config) PrintConfig() {
	// Make a copy to redact secrets and sensitive information.
	tmp := *cfg
	tmp.DBPass = "[REDACTED]"

	// Convert to JSON for nicer formatting.
	cfgJSON, err := json.MarshalIndent(tmp, "", "\t")
	if err != nil {
		klog.Warning("Encountered a problem formatting configuration. ", err)
		klog.Infof("Configuration %#v\n", tmp)
	}
	klog.Infof("Using configuration:\n%s\n", string(cfgJSON))
}

// Simple helper function to read an environment or return a default value
func getEnv(key string, defaultVal string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultVal
}

// Simple helper function to read an environment variable into integer or return a default value
func getEnvAsInt(name string, defaultVal int) int {
	valueStr := getEnv(name, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultVal
}

// Validate required configuration.
func (cfg *Config) Validate() error {
	if cfg.DBName == "" {
		return errors.New("Required environment DB_NAME is not set.")
	}
	if cfg.DBUser == "" {
		return errors.New("Required environment DB_USER is not set.")
	}
	if cfg.DBPass == "" {
		return errors.New("Required environment DB_PASS is not set.")
	}
	return nil
}

// Helper to read an environment variable into a bool or return default value
// func getEnvAsBool(name string, defaultVal bool) bool {
// 	valStr := getEnv(name, "")
// 	if val, err := strconv.ParseBool(valStr); err == nil {
// 		return val
// 	}

// 	return defaultVal
// }

// Helper to read an environment variable into a string slice or return default value
// func getEnvAsSlice(name string, defaultVal []string, sep string) []string {
// 	valStr := getEnv(name, "")

// 	if valStr == "" {
// 		return defaultVal
// 	}

// 	val := strings.Split(valStr, sep)

// 	return val
// }
