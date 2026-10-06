// Copyright Contributors to the Open Cluster Management project

package requestcapture

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stolostron/search-indexer/pkg/config"
)

func TestNewFromConfig_Disabled(t *testing.T) {
	rec := NewFromConfig(&config.Config{RequestCaptureEnabled: false}, nil)
	if rec.Enabled() {
		t.Fatalf("expected disabled recorder")
	}
}

func TestNewFromConfig_FileCapture(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "capture.ndjson")
	body := []byte(`{"hello":"world"}`)

	rec := NewFromConfig(&config.Config{
		RequestCaptureEnabled: true,
		RequestCaptureBackend: "file",
		RequestCaptureFile:    outFile,
		RequestCaptureBuffer:  10,
		RequestCaptureMaxBody: 0,
	}, nil)
	if !rec.Enabled() {
		t.Fatalf("expected enabled recorder")
	}

	rec.Record("cluster-a", "false", "POST", "/aggregator/clusters/cluster-a/sync", "search-indexer.open-cluster-management.svc:3010", body, map[string][]string{"Content-Type": {"application/json"}})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rec.Close(ctx); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	raw, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read capture file: %v", err)
	}

	line := strings.TrimSpace(string(raw))
	if line == "" {
		t.Fatalf("expected captured json line")
	}

	var req capturedRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		t.Fatalf("unmarshal capture line: %v", err)
	}

	if req.ClusterName != "cluster-a" {
		t.Fatalf("unexpected clusterName: %s", req.ClusterName)
	}
	if req.OverwriteStateHeader != "false" {
		t.Fatalf("unexpected overwrite header: %s", req.OverwriteStateHeader)
	}
	if req.Method != "POST" {
		t.Fatalf("unexpected method: %s", req.Method)
	}
	if req.Path != "/aggregator/clusters/cluster-a/sync" {
		t.Fatalf("unexpected path: %s", req.Path)
	}
	if req.BodyBytes != len(body) {
		t.Fatalf("unexpected bodyBytes: %d", req.BodyBytes)
	}
	if req.BodyTruncated {
		t.Fatalf("did not expect truncation")
	}

	decoded, err := base64.StdEncoding.DecodeString(req.BodyBase64)
	if err != nil {
		t.Fatalf("decode base64 body: %v", err)
	}
	if string(decoded) != string(body) {
		t.Fatalf("body mismatch got=%q want=%q", string(decoded), string(body))
	}

	sum := sha256.Sum256(body)
	if req.BodySHA256 != hex.EncodeToString(sum[:]) {
		t.Fatalf("sha mismatch got=%s want=%s", req.BodySHA256, hex.EncodeToString(sum[:]))
	}
}

func TestNewFromConfig_TruncateBody(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "capture.ndjson")
	body := []byte("abcdefghijklmnopqrstuvwxyz")

	rec := NewFromConfig(&config.Config{
		RequestCaptureEnabled: true,
		RequestCaptureBackend: "file",
		RequestCaptureFile:    outFile,
		RequestCaptureBuffer:  10,
		RequestCaptureMaxBody: 5,
	}, nil)
	rec.Record("cluster-b", "true", "POST", "/aggregator/clusters/cluster-b/sync", "", body, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := rec.Close(ctx); err != nil {
		t.Fatalf("close failed: %v", err)
	}

	raw, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read capture file: %v", err)
	}
	line := strings.TrimSpace(string(raw))

	var req capturedRequest
	if err := json.Unmarshal([]byte(line), &req); err != nil {
		t.Fatalf("unmarshal capture line: %v", err)
	}

	if !req.BodyTruncated {
		t.Fatalf("expected truncated body")
	}
	decoded, err := base64.StdEncoding.DecodeString(req.BodyBase64)
	if err != nil {
		t.Fatalf("decode base64 body: %v", err)
	}
	if string(decoded) != "abcde" {
		t.Fatalf("unexpected truncated payload: %q", string(decoded))
	}
}
