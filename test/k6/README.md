# k6 Scale Testing PoC

Load testing for ACM Search using [k6](https://k6.io) with Grafana dashboards.

## Prerequisites

- [k6](https://grafana.com/docs/k6/latest/set-up/install-k6/) installed locally
- [Docker Compose](https://docs.docker.com/compose/install/) for the monitoring stack
- Access to an OpenShift cluster with ACM Search deployed

## Quick Start

```bash
# 1. Start the monitoring stack (Grafana + InfluxDB)
make test-k6-setup

# 2. Create routes if testing against a remote cluster
make test-scale-setup  # indexer route (existing target)
# API route is auto-detected from `oc get route search-api`

# 3. Run a test
make test-k6-indexer K6_VUS=3 K6_DURATION=60s
make test-k6-api K6_VUS=5 K6_DURATION=60s
make test-k6-subscriptions K6_VUS=2 K6_DURATION=60s

# 4. Run all three simultaneously
make test-k6-combined K6_DURATION=120s

# 5. View results at http://localhost:3000 (Grafana, no login required)

# 6. Tear down when done
make test-k6-teardown
```

## Scripts

| Script | What it tests |
|---|---|
| `scripts/indexer-sync.js` | POST sync payloads to the indexer (full state + delta updates) |
| `scripts/api-queries.js` | GraphQL queries: keyword, filter, count, autocomplete, related |
| `scripts/subscriptions.js` | WebSocket GraphQL subscriptions (graphql-transport-ws) |
| `scripts/combined.js` | All three simultaneously with independent VU counts |

## Environment Variables

| Variable | Default | Description |
|---|---|---|
| `INDEXER_HOST` | from Makefile `HOST` | Indexer hostname:port |
| `API_HOST` | auto-detected from `oc get route` | API hostname:port |
| `API_TOKEN` | `oc whoami -t` | Bearer token for API auth |
| `K6_VUS` | `2` | Virtual users (for single-script runs) |
| `K6_DURATION` | `60s` | Test duration |
| `SUB_DURATION` | `30` | Seconds each subscription stays open |
| `INDEXER_VUS` | `3` | Indexer VUs (combined.js) |
| `API_VUS` | `5` | API query VUs (combined.js) |
| `SUB_VUS` | `2` | Subscription VUs (combined.js) |

## Grafana Dashboard

The dashboard auto-provisions when the monitoring stack starts. It shows:

- **Overview**: active VUs, total requests, error rate, data sent
- **Indexer Sync**: request duration (p50/p95) by sync type, throughput
- **API Queries**: query duration by type, throughput breakdown
- **WebSocket Subscriptions**: messages received, inter-message latency, connect time

## Running Without Grafana

k6 outputs a summary to stdout by default. To skip the monitoring stack:

```bash
K6_VUS=2 K6_DURATION=30s INDEXER_HOST=<host> k6 run test/k6/scripts/indexer-sync.js
```
