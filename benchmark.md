# Indexer Benchmarks

## Indexing

  We wanted to compare whether removing keys originally stored in the json data column from the resources table and creating columns for each of the keys would be more efficient than keeping them inside and applying jsonb indexing. 


Our table of interest, <strong>resources</strong>, has <strong>450327</strong> rows.

### <ins><strong>Method 1: Keeping keys in data column and indexing keys using jsonb</strong></ins>

We are looking at the following queries which will need to use different operators depending on what index we choose (using the GIN operator class requires specific operators in query)

----

<strong>Query A (with Gin index):</strong>

`select data->'namespace' as namespace from search.resources where data @> '{"kind" : "Pod"}'`

<strong>Query B (no index):</strong>

`select data->'namespace' from search.resources where data ->> 'kind'= 'Pod'`



We also created an index on two columns using the btree operator and use the following query:

<strong>Query C (BTREE Gin index):</strong>

`select data->'namespace' as namespace from search.resources where data @> '{"kind" : "Pod"}' AND uid = 'someuniqueid';`

which has alternative operator:

<strong>Query D (no index):</strong>

`select data->'namespace' as namespace from search.resources where data ->> 'kind'= 'Pod' AND uid = 'someuniqueid';`


| index on  |  index type | OC  | cost  | query
|---|---|---|---|---|
| no index  |  - | -  | 298 ms | B |
| no index  |  - | -  | 285 ms  | A |
| data  | GIN | jsonb |  276 ms |  A |
| data & kind key  | GIN  |  jsonb |  248 ms |  A |
| key kind  | GIN  | jsonb  | 288 ms  | A|
| no index  | -  |  - |  98 | D |
| key & uid  |  BTREE GIN  |  jsonb  |  97 ms  |    C   |


###### * OC -type of GIN operator class

### <ins>Method 2: Removing json keys and creating columns</ins>

The following two queries were used for testing non-index key columns:

---

<strong>Query D:</strong>

`SELECT namespace from search.resources where kind = 'Pod';`

<strong>Query E:</strong>

`SELECT namespace from search.resources where kind = 'Pod' AND uid = 'someuniqueid';`

| index on  |  index type | OC  | cost  | query
|---|---|---|---|---|
| no index  |  - |  - |  435 ms |  D |
| no index | - | - | 204 ms | E |