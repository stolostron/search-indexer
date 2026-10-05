# Jorge's notes

Table search.request_capture columns
```
id | received_at | cluster | overwrite_state_header | method |  path | host  |headers | body | body_sha256 | body_bytes | body_truncated
```


Query the request_capture table

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

