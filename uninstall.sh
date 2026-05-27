#!/bin/bash

# ============================================================
# IICPC Platform - Uninstall Script
# Usage: ./uninstall.sh [--nuke]
#   (no args)  Remove platform resources, keep minikube running
#   --nuke     Delete everything including minikube cluster
# ============================================================

PLATFORM_NAME="iicpc-platform"
NAMESPACE="iicpc"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()    { echo -e "\033[0;34m[INFO]\033[0m    $1"; }
log_success() { echo -e "${GREEN}[OK]${NC}      $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC}    $1"; }
log_step()    { echo -e "\n${CYAN}━━━ $1 ━━━${NC}"; }

NUKE=false
[ "$1" = "--nuke" ] && NUKE=true

echo ""
echo "=============================================="
echo "   IICPC Platform — Uninstall Script"
echo "   Mode: $([ "$NUKE" = true ] && echo '💥 NUKE EVERYTHING' || echo 'Clean Platform Resources')"
echo "=============================================="
echo ""

if [ "$NUKE" = false ]; then
  echo -e "${YELLOW}This will remove all IICPC platform resources.${NC}"
  echo -e "Use ${RED}--nuke${NC} to also delete the minikube cluster entirely."
  echo ""
  read -p "Continue? [y/N] " CONFIRM
  [[ "$CONFIRM" =~ ^[Yy]$ ]] || { echo "Aborted."; exit 0; }
fi

# -------------------------------------------------------
# DYNAMIC RESOURCES (sandbox pods/svcs created at runtime)
# -------------------------------------------------------
log_step "Dynamic Resources"

log_info "Deleting sandbox pods..."
kubectl delete pods -n "$NAMESPACE" -l app=sandbox    --ignore-not-found=true 2>/dev/null || true

log_info "Deleting sandbox services..."
kubectl delete svc  -n "$NAMESPACE" -l app=sandbox    --ignore-not-found=true 2>/dev/null || true

log_info "Deleting kaniko jobs..."
kubectl delete jobs -n "$NAMESPACE" -l app=kaniko     --ignore-not-found=true 2>/dev/null || true

log_info "Deleting bot-fleet jobs..."
kubectl delete jobs -n "$NAMESPACE" -l app=bot-fleet  --ignore-not-found=true 2>/dev/null || true

log_success "Dynamic resources removed"

# -------------------------------------------------------
# HELM RELEASE
# -------------------------------------------------------
log_step "Helm Release"

if helm list -n "$NAMESPACE" --short 2>/dev/null | grep -q "$PLATFORM_NAME"; then
  helm uninstall "$PLATFORM_NAME" -n "$NAMESPACE"
  log_success "Helm release '$PLATFORM_NAME' removed"
else
  log_warn "No Helm release found — skipping"
fi

# -------------------------------------------------------
# NAMESPACE
# -------------------------------------------------------
log_step "Namespace"

if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
  kubectl delete namespace "$NAMESPACE" --ignore-not-found=true
  log_success "Namespace '$NAMESPACE' deleted"
else
  log_warn "Namespace '$NAMESPACE' not found — skipping"
fi

# -------------------------------------------------------
# MINIKUBE CACHED IMAGES
# -------------------------------------------------------
log_step "Minikube Cached Images"

IMAGES=(
  "mock-exchange"
  "submission-service"
  "bot-fleet"
  "telemetry-service"
  "leaderboard-service"
  "sandbox-runner"
)

for NAME in "${IMAGES[@]}"; do
  minikube image rm "${NAME}:latest" 2>/dev/null \
    && log_info "  Removed: $NAME" \
    || log_warn "  Not cached: $NAME"
done

log_success "Minikube image cache cleared"

# -------------------------------------------------------
# NUKE MODE — delete minikube entirely
# -------------------------------------------------------
if [ "$NUKE" = true ]; then
  log_step "Nuking Minikube"
  log_warn "Deleting minikube cluster entirely..."
  minikube delete
  log_success "minikube cluster deleted"

  log_info "Cleaning up /etc/hosts..."
  sudo sed -i '/iicpc.local/d' /etc/hosts
  log_success "/etc/hosts cleaned"
fi

# -------------------------------------------------------
# DONE
# -------------------------------------------------------
echo ""
echo "=============================================="
echo -e "  ${GREEN}Uninstall Complete!${NC}"
echo "=============================================="
echo ""
if [ "$NUKE" = false ]; then
  echo "  minikube is still running."
  echo "  To redeploy:  ./deploy.sh"
  echo "  To nuke all:  ./uninstall.sh --nuke"
fi
echo ""