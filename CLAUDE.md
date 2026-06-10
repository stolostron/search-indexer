# search-indexer

ACM Search component that receives resource sync events from managed clusters and persists them to PostgreSQL.

For system architecture, data flows, and module layout, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Commands

```bash
make setup          # Generate self-signed TLS cert (required before first run)
make run            # Run locally in development mode
make test           # Unit tests
make coverage       # Unit tests with HTML coverage report
make docker-build   # Build Docker image
make podman-build   # Build with Podman
make lint           # Run golangci-lint + gosec (downloads golangci-lint if not present)
make test-send      # Simulate a sync request against the local instance
```

## Required environment variables

The indexer will exit on startup if these are missing:

| Variable | Description |
|---|---|
| `DB_NAME` | PostgreSQL database name |
| `DB_USER` | PostgreSQL user |
| `DB_PASS` | PostgreSQL password |

Optional overrides (defaults shown): `DB_HOST=localhost`, `DB_PORT=5432`, `AGGREGATOR_ADDRESS=:3010`, `REQUEST_LIMIT=25`, `RESYNC_PERIOD_MS=900000`.

## Non-obvious conventions

- **`-tags development` build tag** enables development mode, which relaxes TLS requirements. Controlled via `pkg/config/config_development.go`, not an env var.
- **TLS cert required** even for local runs. `make setup` generates a self-signed cert at `sslcert/tls.crt` + `sslcert/tls.key` using `sslcert/req.conf`.
- **`X-Overwrite-State` header** controls sync type: `true` = full resync (collector sends complete current state); `false` or absent = delta sync. ResyncData decodes the body as a streaming JSON reader to handle large payloads without loading them fully into memory.
- **Cluster node UID format** is `cluster__<clusterName>` (two underscores). Hub cluster resources carry a `_hubClusterResource` property that triggers cleanup of any old hub cluster data on resync.
- **Database schema** uses the `search` schema, not the default public schema: `search.resources` and `search.edges`.
- **Leader election** (`pkg/clustersync`) uses a Kubernetes lease lock named `search-indexer.open-cluster-management.io`. Only the leader runs the ManagedCluster informers.
- **`make lint`** re-downloads golangci-lint on every run if it is not already installed.

## Fleet Engineering Skills

All skills are available as slash commands. See the [Fleet Engineering skills catalog](https://github.com/OpenShift-Fleet/agentic-sdlc/blob/main/skills/README.md) for the full list with when-to-use guidance.

## Personal configuration

Read personal config at the start of any task that needs an assignee, email, or project key.
Use the tool-aware fallback chain: `~/.config/opencode/user.local.md` (OpenCode),
`.claude/user.local.md` (Claude Code), or `.cursor/rules/user.local.mdc` (Cursor, already in context).
If none exist, fall back to agent memory (`user-config`), then placeholders.
Run `make personalize` to generate all three files (if this repo uses Fleet Engineering tooling).
