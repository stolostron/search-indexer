// Copyright Contributors to the Open Cluster Management project

package database

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_useGoqu(t *testing.T) {
	q, p, er := useGoqu("SELECT uid FROM search.resources WHERE cluster=$1 AND uid!='cluster__$1'", []interface{}{"test-cluster"})

	assert.Equal(t, "SELECT \"uid\" FROM \"search\".\"resources\" WHERE ((\"cluster\" = $1) AND (\"uid\" != $2))", q)
	assert.Equal(t, []interface{}{"test-cluster", "cluster__test-cluster"}, p)
	assert.Nil(t, er)
}

func Test_useGoqu_invalidParams(t *testing.T) {
	q, p, er := useGoqu("INSERT into search.resources values($1,$2,$3) ON CONFLICT (uid) DO UPDATE SET data=$3 WHERE data!=$3", []interface{}{"fakeUid", "fakeCluster"})

	assert.Equal(t, "", q)
	assert.Nil(t, p)
	assert.NotNil(t, er)
}

// Test that UPDATE now requires cluster ownership.
func Test_useGoqu_updateResource_clusterScoped(t *testing.T) {
	q, p, er := useGoqu(
		"UPDATE search.resources SET data=$2 WHERE uid=$1 AND cluster=$3",
		[]interface{}{"my-cluster/abc-123", `{"kind":"Pod"}`, "my-cluster"})

	assert.Nil(t, er)
	assert.Contains(t, q, `"uid" = `)
	assert.Contains(t, q, `"cluster" = `)
	// goqu places SET params before WHERE params in prepared UPDATE statements;
	// the bound slice order is therefore: [data, uid, cluster].
	assert.Equal(t, []interface{}{`{"kind":"Pod"}`, "my-cluster/abc-123", "my-cluster"}, p)
}

func Test_useGoqu_updateResource_wrongParamCount(t *testing.T) {
	q, p, er := useGoqu(
		"UPDATE search.resources SET data=$2 WHERE uid=$1 AND cluster=$3",
		[]interface{}{"uid-only"}) // missing data and cluster

	assert.Equal(t, "", q)
	assert.Nil(t, p)
	assert.NotNil(t, er)
}

// Test that DELETE edges now requires cluster ownership.
func Test_useGoqu_deleteEdge_clusterScoped(t *testing.T) {
	q, p, er := useGoqu(
		"DELETE from search.edges WHERE sourceid=$1 AND destid=$2 AND edgetype=$3 AND cluster=$4",
		[]interface{}{"my-cluster/src", "my-cluster/dst", "ownedBy", "my-cluster"})

	assert.Nil(t, er)
	assert.Contains(t, q, `"sourceid" = `)
	assert.Contains(t, q, `"destid" = `)
	assert.Contains(t, q, `"edgetype" = `)
	assert.Contains(t, q, `"cluster" = `)
	assert.Equal(t, []interface{}{"my-cluster/src", "my-cluster/dst", "ownedBy", "my-cluster"}, p)
}

func Test_useGoqu_deleteEdge_wrongParamCount(t *testing.T) {
	q, p, er := useGoqu(
		"DELETE from search.edges WHERE sourceid=$1 AND destid=$2 AND edgetype=$3 AND cluster=$4",
		[]interface{}{"src", "dst"}) // missing edgeType and cluster

	assert.Equal(t, "", q)
	assert.Nil(t, p)
	assert.NotNil(t, er)
}
