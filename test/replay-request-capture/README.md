# replay-request-capture

Replay captured `search-indexer` sync requests from the `search.request_capture` table.

This tool is useful for scale/perf debugging when you want to:
- capture real request traffic from a scale environment,
- save it locally,
- replay it against a test indexer endpoint.

## 1) Collect capture data in the source environment

First, make sure request capture is enabled on the `search-indexer` deployment:

- `REQUEST_CAPTURE_ENABLED=true`
- `REQUEST_CAPTURE_BACKEND=postgres`

With that enabled, requests are persisted into `search.request_capture`.

### Optional sanity checks

```bash
oc exec -n <namespace> <search-postgres-pod> -- sh -c \
  'psql -U "$POSTGRESQL_USER" -d "$POSTGRESQL_DATABASE" -c "SELECT count(*) FROM search.request_capture;"'
```

```bash
oc exec -n <namespace> <search-postgres-pod> -- sh -c \
  'psql -U "$POSTGRESQL_USER" -d "$POSTGRESQL_DATABASE" -c "SELECT pg_size_pretty(pg_total_relation_size('"'"'search.request_capture'"'"'));"'
```

## 2) Dump `search.request_capture` from the postgres pod

Recommended (stream directly to local file):

```bash
NS=<namespace>
POD=$(oc get pod -n "$NS" -l app=search-postgres -o jsonpath='{.items[0].metadata.name}')
OUT=search-request-capture-$(date +%Y%m%d-%H%M%S).dmp

oc exec -n "$NS" "$POD" -- sh -c '
  pg_dump -U "$POSTGRESQL_USER" -d "$POSTGRESQL_DATABASE" -t search.request_capture -Fc
' > "$OUT"

ls -lh "$OUT"
```

This creates a PostgreSQL custom-format dump (`.dmp`) on your local machine.

## 3) Restore dump into a local PostgreSQL DB

```bash
createdb replaydb
pg_restore -d replaydb ./search-request-capture-YYYYMMDD-HHMMSS.dmp
```

If needed, verify:

```bash
psql replaydb -c "SELECT count(*) FROM search.request_capture;"
```

## 4) Replay requests with this tool

From repo root:

```bash
go run ./test/replay-request-capture \
  -pg-url "postgres://<user>:<pass>@localhost:5432/replaydb?sslmode=disable" \
  -indexer-base-url "https://localhost:3010" \
  -concurrency 5 \
  -rps 20
```

## Common flags

- `-pg-url` (required): Postgres URL containing restored `search.request_capture`
- `-indexer-base-url` (required): target indexer base URL
- `-start-id`: replay only rows with `id >= start-id`
- `-limit`: max rows to replay (`0` = all)
- `-concurrency`: number of workers (default `1`)
- `-rps`: global rate limit (`0` = unlimited)
- `-timeout`: request timeout (default `30s`)
- `-insecure-skip-verify`: skip TLS verification (default `true`)
- `-verify-sha`: validate `body_sha256` before replay (default `true`)
- `-preserve-timing`: preserve original inter-request timing from `received_at`
- `-dry-run`: print replay actions without sending requests

### Dry-run example

```bash
go run ./test/replay-request-capture \
  -pg-url "postgres://<user>:<pass>@localhost:5432/replaydb?sslmode=disable" \
  -indexer-base-url "https://localhost:3010" \
  -dry-run \
  -limit 50
```

## Notes

- `-concurrency 1` replays in strict row order (`id ASC`).
- Higher concurrency and RPS can trigger `429` responses on the target indexer.
- For lab/self-signed certs, keep `-insecure-skip-verify=true`.
