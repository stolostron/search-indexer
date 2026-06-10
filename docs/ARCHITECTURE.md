# search-indexer Architecture

## Overview

search-indexer is a Go HTTPS service running inside the ACM hub cluster. It is the write path for the ACM Search datastore. Each managed cluster runs a `search-collector` agent that sends sync events to this service; the indexer writes those events to a shared PostgreSQL database that `search-api` reads from.

```
search-collector (per managed cluster)
        │  HTTPS POST /aggregator/clusters/{id}/sync
        ▼
  search-indexer  ──────────────────────────────► PostgreSQL
        │                                          search.resources
        │  (leader only, via Kubernetes informers) search.edges
        └──────── ManagedCluster / ManagedClusterInfo / ManagedClusterAddOn
```

## Packages

| Package | Responsibility |
|---|---|
| `main` | Bootstrap: init config, create DAO, start clustersync (goroutine) and server (goroutine), wait for SIGINT/SIGTERM |
| `pkg/config` | All configuration from environment variables. `Cfg` is a package-level singleton. Development mode is a build tag (`-tags development`), not an env var. |
| `pkg/server` | HTTPS server on `:3010`. Routes: `/liveness`, `/readiness`, `/metrics`, `POST /aggregator/clusters/{id}/sync`. Applies two rate-limiting middlewares. |
| `pkg/database` | PostgreSQL DAO. Uses `pgxpool` for connection pooling. Operates on `search.resources` and `search.edges`. Batches writes for throughput. |
| `pkg/clustersync` | Watches `ManagedCluster`, `ManagedClusterInfo`, and `ManagedClusterAddOn` objects and keeps the `Cluster` pseudo-node in PostgreSQL in sync. Requires leader election. |
| `pkg/model` | Plain Go structs: `Resource`, `Edge`, `SyncEvent`, `SyncResponse`, `SyncError`, `DeleteResourceEvent`. |
| `pkg/metrics` | Prometheus registry and instrumentation helpers (`PrometheusMiddleware`, `SlowLog`, `LogStepDuration`, `RequestSize`). |

## Key data flows

### Delta sync (`X-Overwrite-State: false`)

1. Collector sends a JSON `SyncEvent` with arrays: `AddResources`, `UpdateResources`, `DeleteResources`, `AddEdges`, `DeleteEdges`.
2. `server.SyncResources` decodes the full body and calls `database.DAO.SyncData`.
3. `SyncData` enqueues SQL operations into a `batchWithRetry`, flushes in configurable batch sizes (default 2500), and waits for all batches to complete.
4. Responds with `SyncResponse` containing per-operation counts and the current total resource/edge counts (for collector-side validation).

### Full resync (`X-Overwrite-State: true`)

1. Collector sends the complete current state (can be very large — 20 MB+ threshold for the large-request limiter).
2. Body is passed as a raw `[]byte` to `database.DAO.ResyncData`.
3. `ResyncData` uses a streaming JSON decoder (`json.NewDecoder`) to process `addResources` and `addEdges` without fully buffering the body — avoids memory spikes.
4. After upserting, it bulk-deletes resources and edges whose UIDs are absent from the incoming set.
5. On resync from the hub cluster (detected by `_hubClusterResource` property), a background goroutine cleans up stale data from any prior hub cluster name (hub rename handling).

### Cluster node lifecycle (`pkg/clustersync`)

- Leader-elected: only one indexer pod runs the informers at a time.
- `ManagedCluster` events produce or update a `Cluster` pseudo-node; `ManagedClusterInfo` enriches it (console URL, node count, API endpoint).
- Both object types write to the same UID (`cluster__<clusterName>`), with `addAdditionalProperties` merging fields from the in-memory cache.
- `ManagedClusterAddOn` delete events (specifically `search-collector`) trigger deletion of all cluster resources and edges but preserve the cluster node.
- `ManagedCluster` delete events remove the cluster node plus all resources.
- On startup, `deleteStaleClusterResources` cross-references the database against the live cluster list and prunes orphans.

## Database schema

Tables are in the `search` schema (not the default `public`):

| Table | Columns | Notes |
|---|---|---|
| `search.resources` | `uid TEXT PK`, `cluster TEXT`, `data JSONB` | One row per Kubernetes resource. `data` is a free-form property bag (no fixed schema per kind). |
| `search.edges` | `sourceid TEXT`, `sourcekind TEXT`, `destid TEXT`, `destkind TEXT`, `edgetype TEXT`, `cluster TEXT` | Composite PK on `(sourceid, destid, edgetype)`. Represents relationships between resources. `interCluster` edges are excluded from per-cluster resync edge diffing. |

## Rate limiting

Two independent semaphore-based middlewares protect the database from overload:

- `requestLimiterMiddleware`: caps total concurrent requests (default 25, `REQUEST_LIMIT`).
- `largeRequestLimiterMiddleware`: caps concurrent requests larger than 20 MB (default 5, `LARGE_REQUEST_LIMIT`/`LARGE_REQUEST_SIZE`). Requests below the size threshold bypass this limiter.

## TLS

The server requires real certificates even in development. `make setup` generates a self-signed cert into `sslcert/` using `sslcert/req.conf`. The `-tags development` build tag sets `DevelopmentMode=true`, which changes the fatal error on startup TLS failure to a more descriptive message pointing to `./setup.sh`.

## Design decisions

- **No ORM**: Raw SQL via `pgx`/`goqu`. `goqu` is used for parameterized query construction in the resync path (avoids injection; handles IN-clause with slices).
- **Batch writes over individual statements**: `batchWithRetry` accumulates SQL operations and flushes them via `pgx.Batch` for throughput. Each batch operation tracks its UID for error attribution in `SyncResponse`.
- **In-memory cluster cache** (`pkg/database/cache.go`): `ReadClustersCache` / `WriteClustersCache` used in `addAdditionalProperties` to merge `ManagedCluster` and `ManagedClusterInfo` fields without a second DB round-trip.
- **Streaming resync decoder**: Full resync bodies from large clusters can exceed hundreds of MB. The resync path uses `json.NewDecoder` token-by-token to avoid loading the full body into memory.
