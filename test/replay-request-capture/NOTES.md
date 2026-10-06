# Jorge's notes


## Enable request capture

```
REQUEST_CAPTURE_ENABLED = true

```

## Copy the table locally.
```sh
export NS="open-cluster-management"
export POD=$(oc get pod -n "$NS" -l name=search-postgres -o jsonpath='{.items[0].metadata.name}')
export OUT="search-request-capture-$(date +%Y%m%d-%H%M%S).dmp"

oc exec -n "$NS" "$POD" -- sh -c '
  pg_dump -U "$POSTGRESQL_USER" -d "$POSTGRESQL_DATABASE" -t search.request_capture -Fc
' > "$OUT"

ls -lh "$OUT"
```


## Table schema
```sql
CREATE TABLE IF NOT EXISTS search.request_capture (
    id BIGSERIAL PRIMARY KEY, 
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(), 
    cluster TEXT NOT NULL, 
    overwrite_state_header TEXT, 
    method TEXT NOT NULL, 
    path TEXT NOT NULL, 
    host TEXT, headers JSONB, 
    body BYTEA NOT NULL, 
    body_sha256 TEXT NOT NULL, 
    body_bytes INTEGER NOT NULL, 
    body_truncated BOOLEAN NOT NULL DEFAULT FALSE)
```


## Useful queries

```sql
-- START and END times of the request capture.
SELECT MIN(received_at) AS start_date, MAX(received_at) AS end_date FROM search.request_capture;

-- Select a record
SELECT * from search.request_capture LIMIT 1;

-- Sample row record
SELECT id, received_at, cluster, path, overwrite_state_header, convert_from(body, 'UTF-8') from search.request_capture WHERE cluster='vm01819' LIMIT 1 OFFSET 4;

-- Clusters wth the most requests
SELECT cluster, count(id) as requests from search.request_capture GROUP BY cluster ORDER BY requests DESC LIMIT 10;

-- Full cluster resyncs
SELECT count(id) from search.request_capture WHERE overwrite_state_header='true';

-- Clusters with the most resyncs
SELECT cluster,count(id) as c from search.request_capture WHERE overwrite_state_header='true' GROUP BY cluster ORDER BY c DESC;

-- Resyncs by time
SELECT received_at, cluster from search.request_capture WHERE overwrite_state_header='true' LIMIT 100;

```

