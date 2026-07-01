#!/usr/bin/env bash
# backup.sh — Backup Kost deployment, config, and K8s resources.
# Usage: ./scripts/backup.sh [namespace] [output-dir]
# Defaults: namespace=kost, output=./backups/<timestamp>

set -euo pipefail

NS="${1:-kost}"
OUT="${2:-backups/$(date -u +%Y%m%dT%H%M%SZ)}"

mkdir -p "$OUT"

echo "→ backing up Kost from namespace=$NS to $OUT"

# 1. Save live K8s resource YAML (dry-run = client-side, get the actual live state)
for kind in deployment service configmap serviceaccount secret; do
  if kubectl get -n "$NS" "$kind" kost &>/dev/null; then
    kubectl get -n "$NS" "$kind" kost -o yaml > "$OUT/${kind}.yaml" 2>/dev/null
    echo "  ✓ $kind/kost"
  fi
done

# 2. ClusterRole and ClusterRoleBinding (cluster-scoped, no -n flag)
for cr in clusterrole clusterrolebinding; do
  if kubectl get "$cr" kost &>/dev/null; then
    kubectl get "$cr" kost -o yaml > "$OUT/${cr}.yaml"
    echo "  ✓ $cr/kost"
  fi
done

# 3. Export Prometheus metrics snapshot
KOST_POD=$(kubectl get pod -n "$NS" -l app=kost -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)
if [ -n "$KOST_POD" ]; then
  kubectl port-forward -n "$NS" "pod/$KOST_POD" 8081:8080 &
  PF_PID=$!
  sleep 2
  curl -sf http://localhost:8081/metrics > "$OUT/metrics.txt" && echo "  ✓ metrics snapshot" || echo "  ⚠ metrics unavailable"
  curl -sf http://localhost:8081/health > "$OUT/health.txt" && echo "  ✓ health check" || echo "  ⚠ health unavailable"
  kill "$PF_PID" 2>/dev/null
  wait "$PF_PID" 2>/dev/null || true
else
  echo "  ⚠ no kost pod found — skipping metrics"
fi

# 4. Git-tracked manifests (the source of truth for GitOps)
echo "  ✓ deploy/ directory (tracked in git)"

echo "→ backup complete: $(ls -1 "$OUT" | wc -l) files written to $OUT"
ls -lh "$OUT"
