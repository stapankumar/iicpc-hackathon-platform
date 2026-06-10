#!/bin/bash
# ==============================================
#   IICPC Platform — Uninstall Script
#   Removes all platform traces from this machine
# ==============================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_step()    { echo ""; echo "━━━ $1 ━━━"; }
log_success() { echo -e "${GREEN}[OK]${NC}      $1"; }
log_info()    { echo -e "${BLUE}[INFO]${NC}    $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC}    $1"; }

echo ""
echo "=============================================="
echo "   IICPC Platform — Uninstall"
echo "=============================================="
echo ""

log_step "Helm Release"
helm uninstall iicpc-platform -n iicpc 2>/dev/null \
  && log_success "Helm release removed" \
  || log_warn "No Helm release found — skipping"

log_step "Namespace"
kubectl delete namespace iicpc --ignore-not-found=true > /dev/null \
  && log_success "Namespace 'iicpc' deleted" \
  || log_warn "Namespace not found — skipping"

log_step "Minikube"
minikube stop && log_success "minikube stopped"
minikube delete && log_success "minikube deleted"

log_step "Hosts File"
sudo sed -i '/iicpc.local/d' /etc/hosts
log_success "iicpc.local removed from /etc/hosts"

log_step "Docker Images"
IMAGES=$(docker images --format '{{.Repository}}:{{.Tag}}' | \
  grep -E '^(submission-service|telemetry-service|leaderboard-service|bot-fleet|correctness-harness|mock-exchange):' || true)

if [ -n "$IMAGES" ]; then
  echo "$IMAGES" | xargs docker rmi 2>/dev/null || true
  log_success "Project Docker images removed"
else
  log_warn "No project images found — skipping"
fi

echo ""
echo "=============================================="
echo -e "  ${GREEN}Uninstall complete.${NC}"
echo "  IICPC Platform has been removed."
echo "=============================================="
echo ""