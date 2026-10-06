// Copyright Contributors to the Open Cluster Management project

package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	pgxpool "github.com/jackc/pgx/v4/pgxpool"
	"k8s.io/klog/v2"
)

type replayRow struct {
	ID                   int64
	ReceivedAt           time.Time
	Cluster              string
	OverwriteStateHeader string
	Method               string
	Path                 string
	Host                 string
	HeadersJSON          []byte
	Body                 []byte
	BodySHA256           string
}

type options struct {
	PGURL              string
	IndexerBaseURL     string
	StartID            int64
	Limit              int
	Concurrency        int
	RPS                float64
	Timeout            time.Duration
	InsecureSkipVerify bool
	DryRun             bool
	VerifySHA          bool
	PreserveTiming     bool
	LogEvery           int64
}

type stats struct {
	Sent           atomic.Int64
	Failed         atomic.Int64
	TotalBodyBytes atomic.Int64

	mu          sync.Mutex
	StatusCount map[int]int64
}

func main() {
	klog.InitFlags(nil)

	opts := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.Connect(ctx, opts.PGURL)
	if err != nil {
		klog.Exitf("failed to connect to postgres: %v", err)
	}
	defer pool.Close()

	rows, err := loadRows(ctx, pool, opts.StartID, opts.Limit)
	if err != nil {
		klog.Exitf("failed to load capture rows: %v", err)
	}

	if len(rows) == 0 {
		klog.Info("No rows found in search.request_capture matching the filter")
		return
	}

	klog.Infof("Loaded %d rows from search.request_capture", len(rows))

	if opts.VerifySHA {
		if err := verifyChecksums(rows); err != nil {
			klog.Exitf("checksum verification failed: %v", err)
		}
	}

	httpClient := &http.Client{
		Timeout: opts.Timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.InsecureSkipVerify}, //nolint:gosec // Intentional for test replay against lab/self-signed certs.
		},
	}

	st := &stats{StatusCount: map[int]int64{}}
	start := time.Now()

	if opts.PreserveTiming {
		replayWithOriginalTiming(ctx, httpClient, opts, rows, st)
	} else {
		replayConcurrent(ctx, httpClient, opts, rows, st)
	}

	elapsed := time.Since(start)
	printSummary(rows, st, elapsed)
}

func parseFlags() options {
	var opts options
	flag.StringVar(&opts.PGURL, "pg-url", "", "Postgres connection string with search.request_capture data (restored from search-request-capture.dmp)")
	flag.StringVar(&opts.IndexerBaseURL, "indexer-base-url", "", "Indexer base URL, e.g. https://localhost:3010")
	flag.Int64Var(&opts.StartID, "start-id", 0, "Replay only rows with id >= start-id")
	flag.IntVar(&opts.Limit, "limit", 0, "Maximum number of rows to replay (0 means all)")
	flag.IntVar(&opts.Concurrency, "concurrency", 1, "Replay workers (ignored when preserve-timing=true)")
	flag.Float64Var(&opts.RPS, "rps", 0, "Global request rate limit (0 means unlimited)")
	flag.DurationVar(&opts.Timeout, "timeout", 30*time.Second, "Per-request timeout")
	flag.BoolVar(&opts.InsecureSkipVerify, "insecure-skip-verify", true, "Skip TLS verification for indexer endpoint")
	flag.BoolVar(&opts.DryRun, "dry-run", false, "Read and print replay operations without sending HTTP requests")
	flag.BoolVar(&opts.VerifySHA, "verify-sha", true, "Verify body SHA256 against body_sha256 column before replay")
	flag.BoolVar(&opts.PreserveTiming, "preserve-timing", false, "Replay with original inter-request timing based on received_at")
	flag.Int64Var(&opts.LogEvery, "log-every", 1000, "Log progress every N sent requests")
	flag.Parse()

	if opts.PGURL == "" {
		klog.Exit("missing required flag: -pg-url")
	}
	if opts.IndexerBaseURL == "" {
		klog.Exit("missing required flag: -indexer-base-url")
	}
	if _, err := url.ParseRequestURI(opts.IndexerBaseURL); err != nil {
		klog.Exitf("invalid -indexer-base-url: %v", err)
	}
	if opts.Concurrency < 1 {
		opts.Concurrency = 1
	}
	if opts.LogEvery < 1 {
		opts.LogEvery = 1000
	}

	return opts
}

func loadRows(ctx context.Context, pool *pgxpool.Pool, startID int64, limit int) ([]replayRow, error) {
	query := `
SELECT
	id,
	received_at,
	cluster,
	COALESCE(overwrite_state_header, ''),
	COALESCE(method, 'POST'),
	COALESCE(path, ''),
	COALESCE(host, ''),
	COALESCE(headers::text, '{}'),
	body,
	COALESCE(body_sha256, '')
FROM search.request_capture
WHERE id >= $1
ORDER BY id ASC`

	args := []interface{}{startID}
	if limit > 0 {
		query = query + "\nLIMIT $2"
		args = append(args, limit)
	}

	rows, err := pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query rows: %w", err)
	}
	defer rows.Close()

	var out []replayRow
	for rows.Next() {
		var r replayRow
		if err := rows.Scan(
			&r.ID,
			&r.ReceivedAt,
			&r.Cluster,
			&r.OverwriteStateHeader,
			&r.Method,
			&r.Path,
			&r.Host,
			&r.HeadersJSON,
			&r.Body,
			&r.BodySHA256,
		); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		out = append(out, r)
	}

	if rows.Err() != nil {
		return nil, rows.Err()
	}

	return out, nil
}

func verifyChecksums(rows []replayRow) error {
	bad := 0
	for _, r := range rows {
		if r.BodySHA256 == "" {
			continue
		}
		sum := sha256.Sum256(r.Body)
		actual := hex.EncodeToString(sum[:])
		if !strings.EqualFold(actual, strings.TrimSpace(r.BodySHA256)) {
			bad++
			if bad <= 10 {
				klog.Warningf("checksum mismatch row id=%d cluster=%s expected=%s actual=%s", r.ID, r.Cluster, r.BodySHA256, actual)
			}
		}
	}

	if bad > 0 {
		return fmt.Errorf("%d rows failed checksum validation", bad)
	}

	return nil
}

func replayWithOriginalTiming(ctx context.Context, client *http.Client, opts options, rows []replayRow, st *stats) {
	for idx, row := range rows {
		if idx > 0 {
			delta := row.ReceivedAt.Sub(rows[idx-1].ReceivedAt)
			if delta > 0 {
				select {
				case <-time.After(delta):
				case <-ctx.Done():
					return
				}
			}
		}

		sendOne(ctx, client, opts, row, st)
	}
}

func replayConcurrent(ctx context.Context, client *http.Client, opts options, rows []replayRow, st *stats) {
	var wg sync.WaitGroup
	jobs := make(chan replayRow)

	for i := 0; i < opts.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range jobs {
				sendOne(ctx, client, opts, row, st)
			}
		}()
	}

	interval, ticker := rateTicker(opts.RPS)
	if interval > 0 {
		defer ticker.Stop()
	}

	for _, row := range rows {
		if ctx.Err() != nil {
			break
		}
		if ticker != nil {
			select {
			case <-ticker.C:
			case <-ctx.Done():
				break
			}
		}
		jobs <- row
	}

	close(jobs)
	wg.Wait()
}

func rateTicker(rps float64) (time.Duration, *time.Ticker) {
	if rps <= 0 {
		return 0, nil
	}
	interval := time.Duration(float64(time.Second) / rps)
	if interval <= 0 {
		interval = time.Nanosecond
	}
	return interval, time.NewTicker(interval)
}

func sendOne(ctx context.Context, client *http.Client, opts options, row replayRow, st *stats) {
	method := strings.ToUpper(strings.TrimSpace(row.Method))
	if method == "" {
		method = http.MethodPost
	}

	path := strings.TrimSpace(row.Path)
	if path == "" {
		path = "/aggregator/clusters/" + row.Cluster + "/sync"
	}

	targetURL := strings.TrimRight(opts.IndexerBaseURL, "/") + path

	headers := map[string][]string{}
	if len(row.HeadersJSON) > 0 {
		if err := json.Unmarshal(row.HeadersJSON, &headers); err != nil {
			klog.Warningf("row id=%d: invalid headers JSON: %v", row.ID, err)
		}
	}
	if row.OverwriteStateHeader != "" && len(headers["X-Overwrite-State"]) == 0 {
		headers["X-Overwrite-State"] = []string{row.OverwriteStateHeader}
	}

	if opts.DryRun {
		klog.Infof("dry-run id=%d method=%s url=%s bodyBytes=%d", row.ID, method, targetURL, len(row.Body))
		countSent(st, opts, len(row.Body))
		return
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, bytes.NewReader(row.Body))
	if err != nil {
		st.Failed.Add(1)
		klog.Warningf("row id=%d: create request error: %v", row.ID, err)
		return
	}

	for key, vals := range headers {
		for _, val := range vals {
			req.Header.Add(key, val)
		}
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		st.Failed.Add(1)
		klog.Warningf("row id=%d cluster=%s: request failed: %v", row.ID, row.Cluster, err)
		return
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	st.mu.Lock()
	st.StatusCount[resp.StatusCode] = st.StatusCount[resp.StatusCode] + 1
	st.mu.Unlock()

	if resp.StatusCode >= 400 {
		st.Failed.Add(1)
		klog.Warningf("row id=%d cluster=%s: status=%d", row.ID, row.Cluster, resp.StatusCode)
	}

	countSent(st, opts, len(row.Body))
}

func countSent(st *stats, opts options, bodyBytes int) {
	st.Sent.Add(1)
	st.TotalBodyBytes.Add(int64(bodyBytes))
	sent := st.Sent.Load()
	if opts.LogEvery > 0 && sent%opts.LogEvery == 0 {
		klog.Infof("progress sent=%d failed=%d", sent, st.Failed.Load())
	}
}

func printSummary(rows []replayRow, st *stats, elapsed time.Duration) {
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Replay complete\n")
	fmt.Printf("rows loaded:        %d\n", len(rows))
	fmt.Printf("requests sent:      %d\n", st.Sent.Load())
	fmt.Printf("failed requests:    %d\n", st.Failed.Load())
	fmt.Printf("bytes sent:         %s\n", humanBytes(st.TotalBodyBytes.Load()))
	fmt.Printf("elapsed:            %s\n", elapsed.Round(time.Millisecond))

	st.mu.Lock()
	keys := make([]int, 0, len(st.StatusCount))
	for code := range st.StatusCount {
		keys = append(keys, code)
	}
	sort.Ints(keys)
	fmt.Println("status counts:")
	for _, code := range keys {
		fmt.Printf("  %d: %d\n", code, st.StatusCount[code])
	}
	st.mu.Unlock()
	fmt.Println("------------------------------------------------------------")
}

func humanBytes(n int64) string {
	if n < 1024 {
		return strconv.FormatInt(n, 10) + " B"
	}
	units := []string{"KB", "MB", "GB", "TB"}
	f := float64(n)
	for _, u := range units {
		f = f / 1024
		if f < 1024 {
			return fmt.Sprintf("%.2f %s", f, u)
		}
	}
	return fmt.Sprintf("%.2f PB", f/1024)
}
