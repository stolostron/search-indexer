// Copyright Contributors to the Open Cluster Management project

package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgconn"
	pgx "github.com/jackc/pgx/v4"
	"k8s.io/klog/v2"
)

type sharedBatchItem struct {
	batchItem
	onResult func(pgconn.CommandTag) // nil for most items; DELETE items use this for RowsAffected routing
}

type SharedBatch struct {
	mu        sync.Mutex
	dao       *DAO
	batchSize int
	timeout   time.Duration

	items   []sharedBatchItem
	waiters []chan error
	timer   *time.Timer
}

func NewSharedBatch(dao *DAO, batchSize int, timeout time.Duration) *SharedBatch {
	return &SharedBatch{
		dao:       dao,
		batchSize: batchSize,
		timeout:   timeout,
		items:     make([]sharedBatchItem, 0),
		waiters:   make([]chan error, 0),
	}
}

func (sb *SharedBatch) SubmitAndWait(ctx context.Context, items []sharedBatchItem) error {
	if len(items) == 0 {
		return nil
	}

	ch := make(chan error, 1)

	sb.mu.Lock()
	sb.items = append(sb.items, items...)
	sb.waiters = append(sb.waiters, ch)

	if sb.timer == nil {
		sb.timer = time.AfterFunc(sb.timeout, func() {
			sb.mu.Lock()
			sb.fire()
			sb.mu.Unlock()
		})
	}

	if len(sb.items) >= sb.batchSize {
		sb.fire()
	}

	sb.mu.Unlock()

	// If ctx cancels, the caller's items remain in the buffer and will still execute
	// when the batch fires. This is safe — all operations are idempotent (INSERT ON
	// CONFLICT, UPDATE with WHERE, DELETE with cluster guard) so the caller's retry
	// on the next sync cycle produces the same result.
	select {
	case err := <-ch:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// fire snapshots the current items and waiters, resets the buffer, and spawns
// a goroutine to execute the batch and notify waiters. Must be called with mu held.
func (sb *SharedBatch) fire() {
	if len(sb.items) == 0 {
		return
	}

	if sb.timer != nil {
		sb.timer.Stop()
		sb.timer = nil
	}

	items := sb.items
	waiters := sb.waiters
	sb.items = make([]sharedBatchItem, 0)
	sb.waiters = make([]chan error, 0)

	klog.V(4).Infof("SharedBatch firing with %d items for %d callers.", len(items), len(waiters))

	go sb.executeBatch(items, waiters)
}

func (sb *SharedBatch) executeBatch(items []sharedBatchItem, waiters []chan error) {
	var firstErr error

	defer func() {
		if r := recover(); r != nil {
			klog.Errorf("SharedBatch executeBatch panic: %v", r)
			if firstErr == nil {
				firstErr = fmt.Errorf("executeBatch panic: %v", r)
			}
		}
		for _, ch := range waiters {
			ch <- firstErr
		}
	}()

	batch := &pgx.Batch{}
	for _, item := range items {
		batch.Queue(item.query, item.args...)
	}

	br := sb.dao.pool.SendBatch(context.Background(), batch)

	tags := make([]pgconn.CommandTag, len(items))
	for i := range items {
		tag, err := br.Exec()
		tags[i] = tag
		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	closeErr := br.Close()
	if closeErr != nil && firstErr == nil {
		firstErr = closeErr
	}

	if firstErr != nil {
		klog.Warningf("SharedBatch executeBatch error: %v", firstErr)
	} else {
		for i, item := range items {
			if item.onResult != nil {
				item.onResult(tags[i])
			}
		}
	}
}
