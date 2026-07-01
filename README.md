# Kost

**Kubernetes cost anomaly detection + right-sizing agent.**

Deploy once. Get JSON reports of over-provisioned workloads with exact `kubectl` fix commands. No dashboard to maintain, no SaaS to configure.

---

## Quick Start

```bash
# Create namespace and deploy
kubectl create ns kost
kubectl apply -f deploy/rbac.yaml -f deploy/configmap.yaml \
              -f deploy/deployment.yaml -f deploy/service.yaml

# Check the first report
kubectl logs -n kost deployment/kost --tail=20
```

Output:

```json
{"timestamp":"2026-07-01T10:15:02Z","cluster":"prod","total_waste_monthly":2847.00,"findings":[...],"healthy":false}
```

---

## How It Works

1. Polls the Kubernetes Metrics API and pod specs every 15 minutes
2. Compares actual CPU/memory usage to each container's `resources.requests`
3. Flags workloads where `request > actual × 1.5` and estimated waste exceeds $5/month
4. Resolves pod owners (Deployment, StatefulSet) so fix commands target the right resource
5. Emits a JSON report to stdout and exposes Prometheus metrics on `:8080`

---

## Configuration

| Field | Default | Description |
|-------|---------|-------------|
| `cluster_name` | `"unknown"` | Label for metrics and reports |
| `interval` | `"15m"` | Polling interval (minimum 1s) |
| `cpu_per_core_hour` | `0.0316` | AWS m5 on-demand CPU cost per core-hour ($) |
| `mem_per_gb_hour` | `0.0042` | AWS m5 on-demand memory cost per GB-hour ($) |
| `waste_ratio` | `1.5` | Flag when request exceeds actual by this multiplier |
| `min_waste_dollars` | `5.00` | Minimum monthly waste to include in report |
| `port` | `8080` | HTTP server port for /metrics and /health |

### Via ConfigMap

Edit `deploy/configmap.yaml`, then `kubectl apply -f deploy/configmap.yaml` and restart the pod.

---

## Slack Alerts (Optional)

```bash
kubectl create secret generic kost-slack -n kost \
  --from-literal=SLACK_WEBHOOK_URL="https://hooks.slack.com/services/xxx/xxx/xxx"
```

The env var is wired end-to-end; the Slack reporter logic is deferred. Without it, reports are written to stdout.

---

## Deployment

### Requirements

- Kubernetes 1.21+
- metrics-server installed in the cluster
- The agent runs as a single Deployment pod (~32MB memory, 50m CPU)

### Manifests

| File | Purpose |
|------|---------|
| `deploy/rbac.yaml` | ServiceAccount, ClusterRole (read-only), ClusterRoleBinding |
| `deploy/configmap.yaml` | Default configuration |
| `deployment.yaml` | Agent deployment (runAsNonRoot) |
| `deploy/service.yaml` | Service exposing :8080 for Prometheus scrape |

### Least-Privilege RBAC

The ClusterRole grants only:

- `metrics.k8s.io/pods: get, list` — pod resource usage
- `pods: get, list, watch` — pod specs and owner references
- `nodes, persistentvolumeclaims: get, list` — pricing context (future)

No write, no exec, no secrets, no cluster-scoped delete.

---

## Prometheus Metrics

| Metric | Type | Description |
|--------|------|-------------|
| `kost_over_provisioned_count` | gauge | Number of over-provisioned workloads |
| `kost_waste_dollars` | gauge | Estimated monthly waste in USD |

Default scrape endpoint: `:8080/metrics`.

---

## Development

```bash
make build    # go build -o kost ./cmd/kost/
make image    # docker build -t ghcr.io/nirjar/kost:latest .
make deploy   # kubectl apply all manifests
```

### Prerequisites

- Go 1.22+
- Docker
- Access to a Kubernetes cluster with metrics-server

### Project Structure

```
cmd/kost/main.go              # Entry point, HTTP server, report loop
internal/
├── config/config.go          # JSON config loading and validation
├── k8s/
│   ├── client.go             # Kubernetes + Metrics API client
│   └── quantity.go           # Pod resource quantity parser
├── analyze/analyze.go        # Over-provisioned detection + fix commands
└── report/
    ├── stdout.go             # JSON stdout reporter
    └── metrics.go           # Prometheus /metrics endpoint
deploy/                        # Kubernetes manifests
scripts/backup.sh             # Backup script for deployment state
```

---

## Backup

```bash
./scripts/backup.sh            # backup to ./backups/<timestamp>/
./scripts/backup.sh kost .     # backup to current directory
```

Saves: Deployment, ConfigMap, Service, ServiceAccount, Secret, ClusterRole, ClusterRoleBinding, Prometheus metrics snapshot, health check.

---

## License

MIT
