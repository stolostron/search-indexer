// Copyright Contributors to the Open Cluster Management project

package requestcapture

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stolostron/search-indexer/pkg/config"
	"k8s.io/klog/v2"
)

type ExecFunc func(ctx context.Context, sql string, args ...interface{}) error

type Recorder interface {
	Enabled() bool
	Record(clusterName string, overwriteStateHeader string, method string, path string, host string, body []byte, hdr map[string][]string)
	Close(ctx context.Context) error
}

type noopRecorder struct{}

func (n *noopRecorder) Enabled() bool { return false }

func (n *noopRecorder) Record(_ string, _ string, _ string, _ string, _ string, _ []byte, _ map[string][]string) {
}

func (n *noopRecorder) Close(_ context.Context) error { return nil }

type asyncRecorder struct {
	enabled bool
	queue   chan captureEntry
	done    chan struct{}
	backend string

	execFn ExecFunc

	writer io.WriteCloser
	bufw   *bufio.Writer

	maxBodyBytes int
	dropped      atomic.Uint64

	mu sync.Mutex
}

type captureEntry struct {
	ts                   time.Time
	clusterName          string
	overwriteStateHeader string
	method               string
	path                 string
	host                 string
	body                 []byte
	headers              map[string][]string
}

type capturedRequest struct {
	Timestamp            string              `json:"timestamp"`
	ClusterName          string              `json:"clusterName"`
	OverwriteStateHeader string              `json:"overwriteStateHeader"`
	Method               string              `json:"method"`
	Path                 string              `json:"path"`
	Host                 string              `json:"host,omitempty"`
	BodySHA256           string              `json:"bodySha256"`
	BodyBytes            int                 `json:"bodyBytes"`
	BodyTruncated        bool                `json:"bodyTruncated"`
	BodyBase64           string              `json:"bodyBase64,omitempty"`
	Headers              map[string][]string `json:"headers,omitempty"`
}

func NewFromConfig(cfg *config.Config, execFn ExecFunc) Recorder {
	if cfg == nil || !cfg.RequestCaptureEnabled {
		return &noopRecorder{}
	}

	backend := cfg.RequestCaptureBackend
	if backend == "" {
		backend = "postgres"
	}

	if cfg.RequestCaptureBuffer <= 0 {
		cfg.RequestCaptureBuffer = 200
	}

	rec := &asyncRecorder{
		enabled:      true,
		queue:        make(chan captureEntry, cfg.RequestCaptureBuffer),
		done:         make(chan struct{}),
		backend:      backend,
		execFn:       execFn,
		maxBodyBytes: cfg.RequestCaptureMaxBody,
	}

	switch backend {
	case "postgres":
		if execFn == nil {
			klog.Warning("Request capture disabled: backend=postgres but exec function is nil")
			return &noopRecorder{}
		}
	case "file":
		if err := rec.initWriter(cfg.RequestCaptureFile); err != nil {
			klog.Warningf("Request capture disabled: unable to initialize file writer: %v", err)
			return &noopRecorder{}
		}
	default:
		klog.Warningf("Request capture disabled: unknown backend %q", backend)
		return &noopRecorder{}
	}

	go rec.run()
	klog.Infof("Request capture enabled. backend=%q output=%q buffer=%d maxBody=%d", backend, cfg.RequestCaptureFile, cfg.RequestCaptureBuffer, cfg.RequestCaptureMaxBody)

	return rec
}

func (r *asyncRecorder) Enabled() bool { return r.enabled }

func (r *asyncRecorder) initWriter(filePath string) error {
	if filePath == "" {
		r.writer = os.Stdout
		r.bufw = bufio.NewWriterSize(os.Stdout, 1024*1024)
		return nil
	}

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open capture file %q: %w", filePath, err)
	}

	r.writer = f
	r.bufw = bufio.NewWriterSize(f, 1024*1024)

	return nil
}

func (r *asyncRecorder) Record(clusterName string, overwriteStateHeader string, method string, path string, host string, body []byte, hdr map[string][]string) {
	if !r.enabled {
		return
	}

	entry := captureEntry{
		ts:                   time.Now().UTC(),
		clusterName:          clusterName,
		overwriteStateHeader: overwriteStateHeader,
		method:               method,
		path:                 path,
		host:                 host,
		body:                 body,
		headers:              cloneHeaders(hdr),
	}

	select {
	case r.queue <- entry:
	default:
		dropped := r.dropped.Add(1)
		if dropped == 1 || dropped%100 == 0 {
			klog.Warningf("request capture queue full; dropped=%d", dropped)
		}
	}
}

func (r *asyncRecorder) run() {
	flushTicker := time.NewTicker(2 * time.Second)
	defer flushTicker.Stop()

	for {
		select {
		case entry, ok := <-r.queue:
			if !ok {
				r.flushAndClose()
				close(r.done)
				return
			}
			r.writeEntry(entry)
		case <-flushTicker.C:
			if r.backend == "file" {
				r.mu.Lock()
				if err := r.bufw.Flush(); err != nil {
					klog.Warningf("request capture flush failed: %v", err)
				}
				r.mu.Unlock()
			}
		}
	}
}

func (r *asyncRecorder) writeEntry(entry captureEntry) {
	body := entry.body
	bodyLen := len(body)
	truncated := false
	if r.maxBodyBytes > 0 && len(body) > r.maxBodyBytes {
		body = body[:r.maxBodyBytes]
		truncated = true
	}

	hash := sha256.Sum256(entry.body)
	req := capturedRequest{
		Timestamp:            entry.ts.Format(time.RFC3339Nano),
		ClusterName:          entry.clusterName,
		OverwriteStateHeader: entry.overwriteStateHeader,
		Method:               entry.method,
		Path:                 entry.path,
		Host:                 entry.host,
		BodySHA256:           hex.EncodeToString(hash[:]),
		BodyBytes:            bodyLen,
		BodyTruncated:        truncated,
		Headers:              entry.headers,
	}

	if r.backend == "postgres" {
		r.writeEntryToPostgres(entry.ts, req, body)
		return
	}

	req.BodyBase64 = base64.StdEncoding.EncodeToString(body)
	line, err := json.Marshal(req)
	if err != nil {
		klog.Warningf("request capture marshal failed: %v", err)
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, err := r.bufw.Write(line); err != nil {
		klog.Warningf("request capture write failed: %v", err)
		return
	}
	if err := r.bufw.WriteByte('\n'); err != nil {
		klog.Warningf("request capture newline write failed: %v", err)
	}
}

func (r *asyncRecorder) writeEntryToPostgres(receivedAt time.Time, req capturedRequest, body []byte) {
	headersJSON, err := json.Marshal(req.Headers)
	if err != nil {
		klog.Warningf("request capture headers marshal failed: %v", err)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	const sql = `INSERT INTO search.request_capture
	(received_at, cluster, overwrite_state_header, method, path, host, headers, body, body_sha256, body_bytes, body_truncated)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`

	if err := r.execFn(ctx, sql,
		receivedAt,
		req.ClusterName,
		req.OverwriteStateHeader,
		req.Method,
		req.Path,
		req.Host,
		headersJSON,
		body,
		req.BodySHA256,
		req.BodyBytes,
		req.BodyTruncated,
	); err != nil {
		klog.Warningf("request capture postgres insert failed: %v", err)
	}
}

func (r *asyncRecorder) flushAndClose() {
	if r.backend != "file" {
		return
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.bufw != nil {
		if err := r.bufw.Flush(); err != nil {
			klog.Warningf("request capture flush failed during close: %v", err)
		}
	}

	if r.writer != nil && r.writer != os.Stdout {
		if err := r.writer.Close(); err != nil {
			klog.Warningf("request capture writer close failed: %v", err)
		}
	}
}

func (r *asyncRecorder) Close(ctx context.Context) error {
	if !r.enabled {
		return nil
	}

	close(r.queue)

	select {
	case <-r.done:
		if dropped := r.dropped.Load(); dropped > 0 {
			klog.Warningf("request capture stopped with dropped=%d", dropped)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func cloneHeaders(src map[string][]string) map[string][]string {
	if src == nil {
		return nil
	}
	out := make(map[string][]string, len(src))
	for k, v := range src {
		cp := make([]string, len(v))
		copy(cp, v)
		out[k] = cp
	}
	return out
}
