// Copyright Contributors to the Open Cluster Management project

package database

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_QueueWithErrors(t *testing.T) {
	mock := &batchWithRetry{
		connError:   errors.New("failed to connect"),
		connErrorMu: &sync.RWMutex{},
	}

	result := mock.Queue(batchItem{})

	assert.NotNil(t, result)
}
