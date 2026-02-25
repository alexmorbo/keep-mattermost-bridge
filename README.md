# keep-mattermost-bridge

A webhook bridge service that connects [Keep](https://www.keephq.dev/) (AIOps/alerting platform) with [Mattermost](https://mattermost.com/) (team communication). It receives alert webhooks from Keep, posts them as interactive messages to Mattermost, and syncs acknowledgement/resolution actions back to Keep in real time.

---

## Table of Contents

- [Overview](#overview)
- [How It Works](#how-it-works)
- [Prerequisites](#prerequisites)
- [Quick Start](#quick-start)
- [Configuration Reference](#configuration-reference)
  - [Environment Variables](#environment-variables)
  - [Config File](#config-file)
- [API Endpoints](#api-endpoints)
- [Auto Setup (Keep Provider and Workflow)](#auto-setup-keep-provider-and-workflow)
- [Deployment](#deployment)
  - [Local / Binary](#local--binary)
  - [Docker](#docker)
- [Observability](#observability)
- [Troubleshooting](#troubleshooting)

---

## Overview

`keep-mattermost-bridge` (kmbridge) is a bidirectional integration service written in Go. It acts as the glue layer between Keep's alert lifecycle and Mattermost's interactive messaging:

- Receives Keep alert webhooks and posts formatted messages to the appropriate Mattermost channel based on alert severity.
- Handles interactive button clicks (Acknowledge / Resolve / Unacknowledge) from Mattermost and propagates the action back to Keep via enrichments.
- Optionally polls Keep on a configurable interval to detect out-of-band changes made directly in the Keep UI (e.g. manual assignee change) and keeps Mattermost posts in sync.
- On startup, can automatically register itself as a webhook provider and create the corresponding workflow in Keep.

Storage is backed by Valkey (Redis-compatible) to persist the mapping between Keep alert fingerprints and Mattermost post IDs across restarts.

---

## How It Works

### Alert Flow

```
Keep alert fires
      │
      ▼
Keep sends POST /api/v1/webhook/alert
      │
      ▼
Bridge routes alert to Mattermost channel (by severity)
Posts formatted message with [Acknowledge] [Resolve] buttons
      │
      ▼
User clicks button in Mattermost
      │
      ▼
Mattermost POSTs to /api/v1/callback
      │
      ├── Bridge updates Keep via enrichment API
      │   (sets assignee, status)
      │
      └── Bridge updates Mattermost post
          (reflects new status, removes/changes buttons)
      │
      ▼
Keep sends webhook with updated status
      │
      ▼
Bridge updates Mattermost post again (final state)
If resolved: thread reply posted, mapping removed from storage
```

### Polling (optional)

When `POLLING_ENABLED=true`, a background goroutine periodically fetches the list of active alerts from Keep and compares their current assignee/status against the locally stored state. If a discrepancy is detected (indicating a direct change in the Keep UI), the corresponding Mattermost post is updated and a thread reply is appended.

### Alert Statuses and Visual Representation

| Status | Visual |
|---|---|
| Firing | severity-colored attachment, Acknowledge + Resolve buttons |
| Acknowledged | blue attachment, assignee shown, 👀 label |
| Resolved | green attachment, ✅ label, thread reply posted |
| Suppressed | grey attachment, 🔇 label |
| Pending | yellow attachment, ⏳ label |
| Maintenance | purple attachment, 🔧 label |

### Severity Routing

Alerts are routed to Mattermost channels based on the severity field in the Keep webhook payload. Configurable per severity in the config file; a `default_channel_id` is used for unmapped severities.

---

## Prerequisites

Before deploying the bridge you need:

- **Keep** instance accessible over HTTP/HTTPS with a valid API key.
- **Mattermost** instance with a bot account. The bot must be a member of every channel the bridge will post to. Create a bot and copy its access token from **System Console > Integrations > Bot Accounts**.
- **Valkey or Redis** (v7+) instance reachable from the bridge.
- A **publicly reachable URL** for the bridge's callback endpoint (`/api/v1/callback`) so Mattermost can send interactive button payloads to it. TLS is strongly recommended.
- Keep must also be able to reach the bridge's `/api/v1/webhook/alert` endpoint. When `KEEP_SETUP_ENABLED=true` this is derived automatically from `CALLBACK_URL`.

---

## Quick Start

1. Create a bot account in Mattermost and note the token.
2. Generate a Keep API key (Settings > API Keys).
3. Start a Valkey or Redis instance.
4. Create a minimal `config.yaml` (see [Config File](#config-file)).
5. Run the bridge:

```bash
docker run -p 8080:8080 \
  -e MATTERMOST_URL=https://mattermost.example.com \
  -e MATTERMOST_TOKEN=your-bot-token \
  -e KEEP_URL=https://keep.example.com \
  -e KEEP_API_KEY=your-api-key \
  -e KEEP_UI_URL=https://keep.example.com \
  -e CALLBACK_URL=https://kmbridge.example.com/api/v1/callback \
  -e REDIS_ADDR=valkey:6379 \
  -v $(pwd)/config.yaml:/etc/kmbridge/config.yaml \
  keep-mattermost-bridge:latest
```

6. Verify the service is up:

```bash
curl https://kmbridge.example.com/health/live
# {"status":"ok"}
```

7. If `KEEP_SETUP_ENABLED=true` (the default), the bridge automatically registers the Keep webhook provider and workflow on startup. Otherwise configure Keep manually to send webhook events to `https://kmbridge.example.com/api/v1/webhook/alert`.

---

## Configuration Reference

Configuration is provided through environment variables (required and runtime options) and a YAML config file (channel routing, message appearance, label handling).

### Environment Variables

#### Required

| Variable | Description | Example |
|---|---|---|
| `MATTERMOST_URL` | Mattermost API base URL | `https://mattermost.example.com` |
| `MATTERMOST_TOKEN` | Mattermost bot access token | `abc123xyz` |
| `KEEP_URL` | Keep API base URL | `https://keep.example.com` |
| `KEEP_API_KEY` | Keep API key | `keep-api-key` |
| `KEEP_UI_URL` | Keep UI URL (used to build alert deep-links) | `https://keep.example.com` |
| `CALLBACK_URL` | Public URL of the bridge's callback endpoint | `https://kmbridge.example.com/api/v1/callback` |
| `REDIS_ADDR` | Valkey/Redis address | `localhost:6379` |
| `CONFIG_PATH` | Path to the YAML config file | `/etc/kmbridge/config.yaml` |

#### Optional

| Variable | Default | Description |
|---|---|---|
| `SERVER_PORT` | `8080` | HTTP server listen port |
| `LOG_LEVEL` | `info` | Log verbosity: `debug`, `info`, `warn`, `error` |
| `REDIS_PASSWORD` | _(empty)_ | Valkey/Redis password |
| `REDIS_DB` | `0` | Valkey/Redis database number |
| `POLLING_ENABLED` | `false` | Enable background polling for out-of-band Keep changes |
| `POLLING_INTERVAL` | `1m` | How often to poll Keep (minimum: `10s`) |
| `POLLING_ALERTS_LIMIT` | `1000` | Maximum alerts fetched per poll cycle |
| `POLLING_TIMEOUT` | `30s` | Per-cycle timeout for the polling request |
| `KEEP_SETUP_ENABLED` | `true` | Auto-register webhook provider and workflow in Keep on startup |

### Config File

Default path: `/etc/kmbridge/config.yaml`. Override with `CONFIG_PATH`.

```yaml
# Channel routing by severity. First matching severity wins.
# Unmapped severities fall back to default_channel_id.
channels:
  routing:
    - severity: "critical"
      channel_id: "CHANNEL_ID_CRITICAL"
    - severity: "high"
      channel_id: "CHANNEL_ID_HIGH"
    - severity: "warning"
      channel_id: "CHANNEL_ID_WARNINGS"
  default_channel_id: "CHANNEL_ID_DEFAULT"

# Message appearance configuration.
message:
  colors:
    critical: "#CC0000"
    high: "#FF6600"
    warning: "#FFCC00"
    info: "#0099CC"
    low: "#999999"
    acknowledged: "#3399FF"
    resolved: "#33CC33"
    suppressed: "#999999"
    pending: "#FFCC00"
    maintenance: "#9933FF"
  emoji:
    critical: "🔴"
    high: "🟠"
    warning: "🟡"
    info: "🔵"
    low: "⚪"
    acknowledged: "👀"
    resolved: "✅"
    suppressed: "🔇"
    pending: "⏳"
    maintenance: "🔧"
  # Footer shown at the bottom of every Mattermost attachment.
  footer:
    text: "Keep AIOps"
    icon_url: "https://keep.example.com/favicon.ico"
  # Field display options.
  fields:
    show_severity: true
    show_description: true
    # Where to place the severity field: first | after_display | last
    severity_position: "first"

# Label handling.
labels:
  # Labels to show, in this order. If empty, all labels are shown.
  display: ["alertgroup", "container", "node", "namespace", "pod"]
  # Labels never shown, regardless of display list.
  exclude: ["__name__", "prometheus", "alertname", "job", "instance"]
  # Override display name for a label.
  rename:
    alertgroup: "Alert Group"
  # Grouping collapses multiple labels that share a common prefix into a
  # single row when the count of matching labels meets or exceeds threshold.
  grouping:
    enabled: true
    threshold: 2
    groups:
      - prefixes: ["topology_"]
        group_name: "Topology"
        priority: 100
      - prefixes: ["kubernetes_io_", "beta_kubernetes_io_"]
        group_name: "Kubernetes"
        priority: 90
      - prefixes: ["talos_"]
        group_name: "Talos"
        priority: 80

# Map Mattermost usernames to Keep usernames.
# Used when a user acknowledges an alert; their Keep username is sent as the assignee.
# If a mapping is absent, the Mattermost username is used as-is.
users:
  mapping:
    john.doe: "john_keep"
    jane.smith: "jane_keep"

# Polling can also be configured here (overridden by environment variables).
polling:
  enabled: false
  interval: "1m"
  alerts_limit: 1000
  timeout: "30s"

# Keep provider/workflow auto-setup.
setup:
  enabled: true
```

#### Labels Configuration Details

- `display` — controls which labels are rendered in the Mattermost attachment and in what order. If the list is empty, all labels are shown (subject to `exclude`).
- `exclude` — labels on this list are never shown, even if they appear in `display`.
- `rename` — maps a label key to a human-readable display name.
- `grouping` — when the number of labels matching a group's prefixes meets or exceeds `threshold`, they are collapsed into a single grouped row instead of individual fields. Groups are evaluated in descending `priority` order.

---

## API Endpoints

| Method | Path | Description |
|---|---|---|
| `POST` | `/api/v1/webhook/alert` | Receives Keep alert webhook payloads |
| `POST` | `/api/v1/callback` | Receives Mattermost interactive button callbacks |
| `GET` | `/health/live` | Liveness probe — returns `200` when the process is running |
| `GET` | `/health/ready` | Readiness probe — returns `200` when Valkey/Redis is reachable |
| `GET` | `/metrics` | Prometheus/VictoriaMetrics metrics endpoint |

The callback endpoint (`/api/v1/callback`) must be reachable from the Mattermost server. Set `CALLBACK_URL` to its full public URL.

The webhook endpoint (`/api/v1/webhook/alert`) must be reachable from the Keep server. When auto-setup is enabled, this URL is derived from `CALLBACK_URL` by replacing `/callback` with `/webhook/alert`.

---

## Auto Setup (Keep Provider and Workflow)

When `KEEP_SETUP_ENABLED=true` (default), the bridge runs a setup routine at startup:

1. Creates (or updates) a webhook provider in Keep named `kmbridge`.
2. Creates (or updates) a Keep workflow with ID `kmbridge-webhook` that forwards all alert lifecycle events to the bridge's webhook endpoint.

The webhook URL is derived automatically:

```
CALLBACK_URL = https://kmbridge.example.com/api/v1/callback
Webhook URL  = https://kmbridge.example.com/api/v1/webhook/alert
```

If the provider or workflow already exists, the setup step is skipped gracefully. Disable this behavior with `KEEP_SETUP_ENABLED=false` if you manage the Keep configuration externally.

---

## Deployment

### Local / Binary

Requirements: Go 1.24+, a running Valkey/Redis instance.

```bash
# Clone and enter the service directory
cd apps/keep-mattermost-bridge

# Build
make build
# Binary is placed at bin/kmbridge

# Run
export MATTERMOST_URL=https://mattermost.example.com
export MATTERMOST_TOKEN=your-bot-token
export KEEP_URL=https://keep.example.com
export KEEP_API_KEY=your-api-key
export KEEP_UI_URL=https://keep.example.com
export CALLBACK_URL=https://kmbridge.example.com/api/v1/callback
export REDIS_ADDR=localhost:6379
export CONFIG_PATH=./config.yaml

./bin/kmbridge
```

Available `make` targets:

| Target | Description |
|---|---|
| `make build` | Compile binary to `bin/kmbridge` |
| `make test` | Run unit tests |
| `make test-coverage` | Run tests and generate `coverage.html` |
| `make test-integration` | Run integration tests (requires Docker) |
| `make lint` | Run `golangci-lint` |
| `make run` | `go run ./cmd/server` |
| `make clean` | Remove `bin/`, `coverage.out`, `coverage.html` |
| `make docker-build` | Build Docker image tagged `keep-mattermost-bridge:latest` |

### Docker

The image uses a multi-stage build. The final image is based on `distroless/static-debian12:nonroot` — no shell, no package manager, minimal attack surface.

```bash
# Build
docker build -t keep-mattermost-bridge:latest .

# Run
docker run -p 8080:8080 \
  -e MATTERMOST_URL=https://mattermost.example.com \
  -e MATTERMOST_TOKEN=your-bot-token \
  -e KEEP_URL=https://keep.example.com \
  -e KEEP_API_KEY=your-api-key \
  -e KEEP_UI_URL=https://keep.example.com \
  -e CALLBACK_URL=https://kmbridge.example.com/api/v1/callback \
  -e REDIS_ADDR=valkey:6379 \
  -v $(pwd)/config.yaml:/etc/kmbridge/config.yaml \
  keep-mattermost-bridge:latest
```

Build arguments:

| Argument | Default | Description |
|---|---|---|
| `GO_VERSION` | `1.24` | Go toolchain version used in the builder stage |
| `IMAGE_REGISTRY` | `docker.io` | Registry prefix for base images |
| `GOPROXY` | `https://proxy.golang.org,direct` | Go module proxy |


---

## Observability

### Health Probes

| Endpoint | Probe type | Passes when |
|---|---|---|
| `GET /health/live` | Liveness | Process is running |
| `GET /health/ready` | Readiness | Valkey/Redis connection is healthy |

### Metrics

Prometheus/VictoriaMetrics metrics are exposed at `GET /metrics`.

| Metric category | What it covers |
|---|---|
| Alert counters | Alerts received, broken down by severity and status |
| Mattermost API | Request counters and latency histograms per operation |
| Keep API | Request counters and latency histograms per operation |
| Polling | Execution count, error count, and cycle duration |
| Assignee resolution | Retry attempts, successes, and errors when resolving Mattermost user to Keep user |
| Active tracked posts | Gauge of alert-to-post mappings currently held in storage |

### Logging

Structured JSON logs are written to stdout via `slog`. Set `LOG_LEVEL=debug` to see per-request and per-action detail including the raw payloads received from Keep and Mattermost.

---

## Troubleshooting

### The bridge starts but Keep never sends webhooks

Check that the Keep workflow was created. With `KEEP_SETUP_ENABLED=true` the startup log will contain a line indicating whether the provider and workflow were registered or already existed. If auto-setup is disabled, verify the Keep workflow manually points to `https://<bridge-host>/api/v1/webhook/alert`.

Confirm Keep can reach the bridge by checking Keep's outbound webhook delivery logs.

### Mattermost buttons do nothing

The `CALLBACK_URL` must be reachable from the Mattermost server, not just from the client browser. Verify by curling the URL from the Mattermost host:

```bash
curl -X POST https://kmbridge.example.com/api/v1/callback
# Should return 400 (bad request), not a connection error
```

Also confirm the bot token is valid and has not expired.

### Mattermost posts appear in the wrong channel

The `channels.routing` list is matched by exact severity string. Check that the severity values sent by Keep match the keys in your config. Unknown severities fall back to `channels.default_channel_id`. Enable `LOG_LEVEL=debug` to see the severity value extracted from each incoming webhook.

### Readiness probe fails (`/health/ready` returns non-200)

The bridge cannot reach Valkey/Redis. Check `REDIS_ADDR`, `REDIS_PASSWORD`, and `REDIS_DB`. Network policies in Kubernetes may also block the connection; ensure the `kmbridge` namespace can reach the Valkey pod on port 6379.

### Alert updates are not reflected in Mattermost after resolving in Keep UI

This is an out-of-band change that bypasses the webhook flow. Enable polling:

```bash
POLLING_ENABLED=true
POLLING_INTERVAL=1m
```

Polling compares the current Keep state against locally stored state and triggers a Mattermost post update when a difference is found.

### User clicks Acknowledge but Keep shows a different username

The bridge maps Mattermost usernames to Keep usernames via `users.mapping` in the config file. If no mapping exists for a user, the raw Mattermost username is sent to Keep. Add the mapping:

```yaml
users:
  mapping:
    mattermost_username: "keep_username"
```

### Duplicate posts for the same alert

The bridge uses the Keep alert fingerprint as the deduplication key in Valkey/Redis. If the same alert arrives without a consistent fingerprint (e.g. the Keep workflow or alerting rule changed), duplicate posts may appear. Check Keep's alert fingerprint configuration and ensure the workflow sending webhooks includes the `fingerprint` field.
