# Search Indexer — Metrics Reference

The indexer exposes Prometheus metrics at `GET /metrics` (default port `:3010`, TLS).

## Endpoint

| Detail | Value |
|---|---|
| URL | `https://<host>:3010/metrics` |
| Method | `GET` |
| Transport | TLS (cert at `./sslcert/tls.crt`) |
| Prometheus scrape path | `/metrics` |

The server address can be overridden with the `AGGREGATOR_ADDRESS` environment variable.


## Metrics

| Metric | Type | Labels | What it measures |
|---|---|---|---|
| `search_indexer_request_count` | CounterVec | `managed_cluster_name` | Sync requests received per managed cluster |
| `search_indexer_request_duration` | HistogramVec | `code` | Sync request processing latency (seconds) |
| `search_indexer_requests_in_flight` | Gauge | — | Concurrent sync requests currently being processed |
| `search_indexer_request_size` | Histogram | — | Resource changes (add+update+delete) per **delta** sync |
| `search_indexer_db_resource_event_count` | CounterVec | `operation` (`insert`/`update`/`delete`), `kind`, `cluster` | Resource DB events sent to PostgreSQL (deletes counted by rows affected) |

## PromQL

Useful queries to visualize the health of this service.

| Check | PromQL |
|---|---|
| Requests per minute | `rate(search_indexer_request_duration_count[2m])*60` |
| Average request duration | `rate(search_indexer_request_duration_sum[5m])/rate(search_indexer_request_duration_count[5m])` |
| Sync request rate (all clusters) | `sum(rate(search_indexer_request_count[5m]))` |
| p99 sync latency | `histogram_quantile(0.99, sum(rate(search_indexer_request_duration_bucket[5m])) by (le))` |
| Resources written per second | `sum(rate(search_indexer_db_resource_event_count[5m]))` |

---

## Metrics Details

### `search_indexer_request_count` · CounterVec

Incremented once per HTTP request received from a managed cluster.

| Attribute | Value |
|---|---|
| Type | `counter` |
| Labels | `managed_cluster_name` |
| Populated by | `promhttp.InstrumentHandlerCounter` in the middleware |

**When is it recorded?**
Every POST to `/aggregator/clusters/{id}/sync` — both delta syncs and full resyncs — increments this counter by 1 at handler entry, regardless of whether the request succeeds or fails.

**PromQL examples**

```promql
# Overall request rate per second (all clusters)
sum(rate(search_indexer_request_count[5m]))

# Request rate per cluster
rate(search_indexer_request_count[5m])

# Top 5 most active clusters in the last 10 minutes
topk(5, increase(search_indexer_request_count[10m]))

# Clusters that have sent at least one request in the last 15 minutes
count by (managed_cluster_name) (
  increase(search_indexer_request_count[15m]) > 0
)

# Alert: a cluster has not checked in for more than 20 minutes
# (useful to detect a lost collector)
increase(search_indexer_request_count[20m]) == 0
```

---

### `search_indexer_request_duration` · HistogramVec

Wall-clock time (in seconds) spent processing each sync request, from the moment the HTTP handler receives the request to when the response is written.

| Attribute | Value |
|---|---|
| Type | `histogram` |
| Labels | `code` (HTTP response status code, e.g. `"200"`, `"500"`) |
| Buckets (s) | `0.25, 0.5, 1, 1.5, 2, 3, 5, 10` |
| Populated by | `promhttp.InstrumentHandlerDuration` in the middleware |

**When is it recorded?**
Once per handled request. The `code` label lets you separate successful requests from errors at a glance.

**PromQL examples**

```promql
# 99th-percentile latency across all requests (5-minute window)
histogram_quantile(0.99,
  sum(rate(search_indexer_request_duration_bucket[5m])) by (le)
)

# 50th and 95th percentile, broken out by HTTP status code
histogram_quantile(0.50,
  sum(rate(search_indexer_request_duration_bucket[5m])) by (le, code)
)
histogram_quantile(0.95,
  sum(rate(search_indexer_request_duration_bucket[5m])) by (le, code)
)

# Average request duration per second
rate(search_indexer_request_duration_sum[5m])
  / rate(search_indexer_request_duration_count[5m])

# Fraction of requests taking longer than 3 seconds
1 - (
  sum(rate(search_indexer_request_duration_bucket{le="3"}[5m]))
    / sum(rate(search_indexer_request_duration_count[5m]))
)

# Alert: p99 latency above 5 s for 2 consecutive minutes
histogram_quantile(0.99,
  sum(rate(search_indexer_request_duration_bucket[2m])) by (le)
) > 5
```

---

### `search_indexer_requests_in_flight` · Gauge

Number of sync requests being actively processed at any given instant.

| Attribute | Value |
|---|---|
| Type | `gauge` |
| Labels | none |
| Populated by | `promhttp.InstrumentHandlerInFlight` in the middleware |

**When is it recorded?**
The gauge is incremented by `+1` when the handler starts and decremented by `-1` when the handler returns. It reflects the instantaneous concurrency level of the indexer.

**PromQL examples**

```promql
# Current concurrency
search_indexer_requests_in_flight

# Average concurrency over the last 5 minutes
avg_over_time(search_indexer_requests_in_flight[5m])

# Maximum observed concurrency in the last hour
max_over_time(search_indexer_requests_in_flight[1h])

# Alert: concurrency has been above the request limit (default 25) for 1 minute
search_indexer_requests_in_flight > 25
```

---

### `search_indexer_request_size` · Histogram

Number of resource changes (add + update + delete combined) in each **delta sync** request. Gives a distribution of payload size across all delta syncs.

| Attribute | Value |
|---|---|
| Type | `histogram` |
| Labels | none |
| Buckets (resources) | `50, 100, 200, 500, 5000, 10000, 25000, 50000, 100000, 200000` |
| Populated by | `metrics.RequestSize.Observe(...)` in `pkg/server/syncHandler.go` |

**When is it recorded?**
Once per **delta sync** request (`X-Overwrite-State: false`), immediately after the JSON body is decoded. Full resync requests (`X-Overwrite-State: true`) are **not** observed by this metric because they are decoded as a stream and the total is not known upfront.

**PromQL examples**

```promql
# Median (p50) number of changes per delta sync request
histogram_quantile(0.50,
  sum(rate(search_indexer_request_size_bucket[10m])) by (le)
)

# 95th-percentile payload size
histogram_quantile(0.95,
  sum(rate(search_indexer_request_size_bucket[10m])) by (le)
)

# Average changes per delta sync
rate(search_indexer_request_size_sum[5m])
  / rate(search_indexer_request_size_count[5m])

# Total resource changes received in the last hour via delta sync
increase(search_indexer_request_size_sum[1h])

# Alert: a single delta sync carrying more than 50 000 changes (outlier detection)
histogram_quantile(0.99,
  sum(rate(search_indexer_request_size_bucket[5m])) by (le)
) > 50000
```

---

### `search_indexer_db_resource_event_count` · CounterVec

Number of individual resource events sent to PostgreSQL, broken down by operation type, resource kind, and managed cluster. This is the primary signal for understanding the write throughput of the indexer and which clusters or resource kinds are driving load.
For delete operations, the count reflects rows actually deleted by PostgreSQL (`RowsAffected()`), not the number of UIDs requested for deletion.

**IMPORTANT:** You must set ENABLE_DETAILED_METRICS=true to get the kind and cluster labels. Otherwise, only the operation label will be used. This is because high cardinality labels can impact the performance of the Prometheus server.

| Attribute | Value |
|---|---|
| Type | `counter` |
| Labels | `operation` — `"insert"`, `"update"`, `"delete"` |
| | `kind` — Kubernetes resource kind (e.g. `"Pod"`, `"Deployment"`); `""` for bulk resync deletes |
| | `cluster` — name of the cluster that originated the sync event |
| Populated by | `pkg/database/sync.go` (delta sync) and `pkg/database/resync.go` (full resync) |

**When is it recorded?**

| Path | `"insert"` | `"update"` | `"delete"` |
|---|---|---|---|
| **Delta sync** (`X-Overwrite-State: false`) | Per resource in `addResources[]` accepted into the batch queue (kind validated, UID prefix valid) | Per resource in `updateResources[]` accepted into the batch queue (kind validated, UID prefix valid) | Rows actually deleted by PostgreSQL (`RowsAffected()`), not the number of UIDs requested (`kind=""`) |
| **Full resync** (`X-Overwrite-State: true`) | Per incoming resource accepted into the upsert batch queue | _(never incremented; resyncs use upsert semantics)_ | Rows actually deleted by PostgreSQL (`RowsAffected()`) when pruning stale resources (`kind=""`) |

**PromQL examples**

```promql
# --- Operations per minute ---

# All DB events per minute (combined)
sum(rate(search_indexer_db_resource_event_count[1m])) * 60

# DB events per minute, split by operation type
sum by (operation) (rate(search_indexer_db_resource_event_count[1m])) * 60

# Inserts per minute
rate(search_indexer_db_resource_event_count{operation="insert"}[1m]) * 60

# Updates per minute
rate(search_indexer_db_resource_event_count{operation="update"}[1m]) * 60

# Deletes per minute
rate(search_indexer_db_resource_event_count{operation="delete"}[1m]) * 60


# --- Break down by kind or cluster ---

# Top 5 resource kinds by insert rate
topk(5, sum by (kind) (rate(search_indexer_db_resource_event_count{operation="insert"}[5m])))

# DB event rate per managed cluster
sum by (cluster) (rate(search_indexer_db_resource_event_count[5m]))

# Insert rate for a specific cluster
rate(search_indexer_db_resource_event_count{operation="insert", cluster="my-cluster"}[5m]) * 60


# --- Throughput over longer windows ---

# Total DB events in the last hour
sum(increase(search_indexer_db_resource_event_count[1h]))

# Breakdown by operation in the last 24 hours
sum by (operation) (increase(search_indexer_db_resource_event_count[24h]))

# Ratio of deletes to total operations (churn indicator)
sum(rate(search_indexer_db_resource_event_count{operation="delete"}[5m]))
  / sum(rate(search_indexer_db_resource_event_count[5m]))


# --- Alerting ---

# Alert: no resources processed in 10 minutes (collector may be down)
sum(increase(search_indexer_db_resource_event_count[10m])) == 0

# Alert: delete rate spikes above 500/min (unexpected mass deletion)
sum(rate(search_indexer_db_resource_event_count{operation="delete"}[2m])) * 60 > 500

# Alert: insert rate drops to zero while the indexer is still receiving requests
#   (requests coming in but nothing being inserted — possible DB issue)
(
  sum(rate(search_indexer_request_count[5m])) > 0
) and (
  sum(rate(search_indexer_db_resource_event_count{operation="insert"}[5m])) == 0
)
```

---

## Dashboard — Combined PromQL Recipes

These queries compose multiple metrics for holistic views of indexer health.

```promql
# ── Throughput summary ──────────────────────────────────────────────────────

# DB events per second (all ops)
sum(rate(search_indexer_db_resource_event_count[5m]))

# DB events per request (average batch size per sync)
sum(rate(search_indexer_db_resource_event_count[5m]))
  / sum(rate(search_indexer_request_count[5m]))


# ── Latency vs. load ────────────────────────────────────────────────────────

# p95 latency vs. current concurrency (correlation panel)
histogram_quantile(0.95,
  sum(rate(search_indexer_request_duration_bucket[5m])) by (le)
)
# alongside:
search_indexer_requests_in_flight


# ── Error rate ──────────────────────────────────────────────────────────────

# Fraction of requests that returned a non-200 status
# search_indexer_request_count only has the managed_cluster_name label (no code label),
# so use the duration histogram which is keyed by code.
sum(rate(search_indexer_request_duration_count{code!="200"}[5m]))
  / sum(rate(search_indexer_request_duration_count[5m]))

# Absolute 5xx error rate per second
sum(rate(search_indexer_request_duration_count{code=~"5.."}[5m]))


# ── Fleet coverage ──────────────────────────────────────────────────────────

# Number of distinct clusters that have synced in the last 5 minutes
count(count by (managed_cluster_name) (
  increase(search_indexer_request_count[5m]) > 0
))

# Clusters that synced in the last 5 m but not in the previous 5 m
# (newly appearing collectors)
count by (managed_cluster_name) (increase(search_indexer_request_count[5m]) > 0)
  unless
count by (managed_cluster_name) (increase(search_indexer_request_count[5m] offset 5m) > 0)
```

---

## Logging Helpers (not Prometheus)

Two timing helpers in `pkg/metrics/` emit to `klog` rather than Prometheus.
They are documented here so operators know to look in pod logs, not in metrics, for this information.

### `SlowLog`

Logs a warning if a sync or resync handler takes longer than the configured threshold.

```
WARN  3.142s - Slow Sync from cluster production-east.
WARN  8.005s - Slow resync from   production-east.
```

Controlled by the `SLOW_LOG` environment variable (default `1000` ms). Raise the value to reduce log noise on large clusters; lower it to detect regressions earlier.

### `LogStepDuration` (`-v=5`)

Lap-timer for resync sub-steps. Only visible at klog verbosity level 5 (`-v=5`).

```
>  142ms  [production-east] Resync QUERY existing edges
>  891ms  [production-east] Reset edges stats: INSERT [4321] DELETE [12]
```

---

## Environment Variables That Affect Metrics

| Variable | Default | Effect |
|---|---|---|
| `AGGREGATOR_ADDRESS` | `:3010` | Address and port where `/metrics` is served |
| `SLOW_LOG` | `1000` | Threshold in ms above which a sync/resync emits a `klog.Warning`; no effect on Prometheus counters |
| `REQUEST_LIMIT` | `25` | Max concurrent requests; shapes when `search_indexer_requests_in_flight` reaches saturation |
| `LARGE_REQUEST_LIMIT` | `5` | Max concurrent large requests; same indirect effect as above |
| `LARGE_REQUEST_SIZE` | `20971520` | Byte threshold to classify a request as "large"; influences concurrency limits and therefore `search_indexer_requests_in_flight` |
