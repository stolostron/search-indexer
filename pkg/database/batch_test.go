// Copyright Contributors to the Open Cluster Management project

package database

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_QueueWithErrors(t *testing.T) {
	mock := &batchWithRetry{
		connError: errors.New("failed to connect"),
	}

	result := mock.Queue(batchItem{})

	assert.NotNil(t, result)
}
