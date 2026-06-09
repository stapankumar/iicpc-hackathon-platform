#!/bin/bash
set -e

# ============================================================
# IICPC Platform — Deploy Script
#
# Usage:
#   ./deploy.sh                        Incremental deploy (all services)
#   ./deploy.sh --full                 Wipe everything, fresh start
#   ./deploy.sh --service <name>       Rebuild one service only
#
# First time on any machine:
#   ./deploy.sh --full
#
# After editing Go code:
#   ./deploy.sh --service submission-service
# ============================================================

PLATFORM_NAME="iicpc-platform"
NAMESPACE="iicpc"
HELM_CHART="helm/iicpc-platform"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info()    { echo -e "${BLUE}[INFO]${NC}    $1"; }
log_success() { echo -e "${GREEN}[OK]${NC}      $1"; }
log_warn()    { echo -e "${YELLOW}[WARN]${NC}    $1"; }
log_error()   { echo -e "${RED}[ERROR]${NC}   $1"; exit 1; }
log_step()    { echo -e "\n${CYAN}━━━ $1 ━━━${NC}"; }

# -------------------------------------------------------
# IMAGE TAG — unique per build, fixes stale image problem
# Uses git SHA + timestamp so every build is distinct.
# Judges cloning fresh also get a unique tag (no cache).
# -------------------------------------------------------
GIT_SHA=$(git rev-parse --short HEAD 2>/dev/null || echo "nogit")
BUILD_TAG="${GIT_SHA}-$(date +%s)"

FULL_DEPLOY=false
SINGLE_SERVICE=""

while [[ $# -gt 0 ]]; do
  case $1 in
    --full)    FULL_DEPLOY=true; shift ;;
    --service) SINGLE_SERVICE="$2"; shift 2 ;;
    *) log_error "Unknown argument: $1" ;;
  esac
done

echo ""
echo "=============================================="
echo "   IICPC Platform — Deploy Script"
if [ "$FULL_DEPLOY" = true ]; then
  echo "   Mode: FULL WIPE + REDEPLOY"
elif [ -n "$SINGLE_SERVICE" ]; then
  echo "   Mode: SINGLE SERVICE → $SINGLE_SERVICE"
else
  echo "   Mode: INCREMENTAL (all services)"
fi
echo "   Build tag: $BUILD_TAG"
echo "=============================================="
echo ""

# -------------------------------------------------------
# SERVICES MAP
# -------------------------------------------------------
declare -A SERVICE_PATHS=(
  [mock-exchange]="./mock-exchange"
  [submission-service]="./backend/submission-service"
  [bot-fleet]="./backend/bot-fleet"
  [telemetry-service]="./backend/telemetry-service"
  [leaderboard-service]="./backend/leaderboard-service"
  [sandbox-runner]="./backend/sandbox-runner"
  [correctness-harness]="./backend/correctness-harness"
)

# -------------------------------------------------------
# PREFLIGHT
# -------------------------------------------------------
log_step "Preflight Checks"

command -v docker   >/dev/null 2>&1 || log_error "docker not found"
command -v minikube >/dev/null 2>&1 || log_error "minikube not found"
command -v kubectl  >/dev/null 2>&1 || log_error "kubectl not found"
command -v helm     >/dev/null 2>&1 || log_error "helm not found"
command -v git      >/dev/null 2>&1 || log_warn  "git not found — build tag will use 'nogit' prefix"
log_success "All tools present"

# -------------------------------------------------------
# MINIKUBE
# -------------------------------------------------------
log_step "Minikube"

MINIKUBE_STATUS=$(minikube status --format='{{.Host}}' 2>/dev/null || echo "Stopped")
if [ "$MINIKUBE_STATUS" = "Running" ]; then
  log_success "minikube already running"
else
  TOTAL_CPUS=$(nproc)
  TOTAL_MEM_GB=$(free -g | awk '/^Mem:/{print $2}')
  MINIKUBE_CPUS=$(( TOTAL_CPUS * 60 / 100 )); [ "$MINIKUBE_CPUS" -lt 2 ] && MINIKUBE_CPUS=2
  MINIKUBE_MEM_GB=$(( TOTAL_MEM_GB * 60 / 100 )); [ "$MINIKUBE_MEM_GB" -lt 4 ] && MINIKUBE_MEM_GB=4
  log_info "Starting minikube (CPUs: ${MINIKUBE_CPUS}, Mem: ${MINIKUBE_MEM_GB}g)..."
  minikube start --cpus="${MINIKUBE_CPUS}" --memory="${MINIKUBE_MEM_GB}g"
  log_success "minikube started"
fi

if [ -n "$DOCKER_USERNAME" ] && [ -n "$DOCKER_PASSWORD" ]; then
    minikube ssh "docker login -u $DOCKER_USERNAME -p $DOCKER_PASSWORD" >/dev/null 2>&1
    log_success "Docker Hub authenticated inside minikube"
fi

# -------------------------------------------------------
# ADDONS
# -------------------------------------------------------
log_step "Minikube Addons"

for ADDON in ingress metrics-server; do
  minikube addons enable $ADDON >/dev/null 2>&1 \
    && log_success "$ADDON enabled" \
    || log_warn "$ADDON already enabled or skipped"
done

# Pre-load registry image before enabling addon — avoids Docker Hub rate limit
if ! minikube image ls 2>/dev/null | grep -q "registry:3.0.0"; then
    log_info "Pre-loading registry:3.0.0 into minikube..."
    docker pull registry:3.0.0
    minikube image load registry:3.0.0
    log_success "registry:3.0.0 pre-loaded"
fi

# Registry addon — only if not already running
if ! kubectl get pod -n kube-system -l kubernetes.io/minikube-addons=registry \
    --no-headers 2>/dev/null | grep -q Running; then
  if minikube addons enable registry \
    --images='KubeRegistryProxy=gcr.io/google_containers/kube-registry-proxy:0.4'; then
    log_success "registry addon enabled"
    # Force local image — avoids digest-pinned Docker Hub pull
    kubectl patch deployment registry -n kube-system \
      -p '{"spec":{"template":{"spec":{"containers":[{"name":"registry","image":"docker.io/registry:3.0.0","imagePullPolicy":"Never"}]}}}}'
  else
    log_error "registry addon failed to enable — run: export DOCKER_USERNAME=x DOCKER_PASSWORD=y"
  fi
else
  log_success "registry addon already running"
fi

log_info "Waiting for registry pod to be ready..."
kubectl wait pod -n kube-system -l actual-registry=true \
  --for=condition=Ready --timeout=120s
log_success "Registry pod ready"

# -------------------------------------------------------
# PRE-LOAD THIRD-PARTY IMAGES
# Avoids Docker Hub rate limits on every run.
# Only loads if not already in minikube cache.
# -------------------------------------------------------
log_step "Pre-loading Third-party Images"

for IMG in "redis:7.0" "gcr.io/kaniko-project/executor:latest"; do
  if minikube image ls 2>/dev/null | grep -q "$(echo $IMG | cut -d: -f1)"; then
    log_success "$IMG already in minikube cache"
  else
    log_info "Pulling $IMG..."
    docker pull "$IMG"
    minikube image load "$IMG"
    log_success "$IMG loaded"
  fi
done

# Push golang and alpine into internal registry for Kaniko mirror
# This avoids Kaniko pulling from Docker Hub on every submission (saves ~3min)
REGISTRY_IP=$(kubectl get svc registry -n kube-system -o jsonpath='{.spec.clusterIP}')

# Configure Docker to allow insecure push to internal registry
DAEMON_JSON="/etc/docker/daemon.json"
if ! grep -q "$REGISTRY_IP" "$DAEMON_JSON" 2>/dev/null; then
    log_info "Configuring Docker insecure registry for $REGISTRY_IP:80..."
    echo "{\"insecure-registries\": [\"${REGISTRY_IP}:80\"]}" | sudo tee "$DAEMON_JSON" > /dev/null
    sudo systemctl restart docker
    minikube start
    log_success "Docker configured for internal registry"
fi

for IMG in "golang:1.24.0" "alpine:3.19"; do
  INTERNAL_TAG="${REGISTRY_IP}:80/${IMG}"
  if docker pull "${INTERNAL_TAG}" >/dev/null 2>&1; then
    log_success "$IMG already in internal registry"
  else
    log_info "Pulling $IMG from Docker Hub..."
    docker pull "$IMG"
    log_info "Pushing $IMG to internal registry via port-forward..."
    kubectl port-forward -n kube-system service/registry 5000:80 &
    PF_PID=$!
    sleep 3
    docker tag "$IMG" "localhost:5000/library/${IMG}"
    docker push "localhost:5000/library/${IMG}"
    kill $PF_PID 2>/dev/null
    log_success "$IMG pushed to internal registry (Kaniko will use this)"
  fi
done

# -------------------------------------------------------
# /etc/hosts
# -------------------------------------------------------
log_step "Hosts Configuration"

MINIKUBE_IP=$(minikube ip)
if grep -q "iicpc.local" /etc/hosts; then
  sudo sed -i "s/.*iicpc.local/$MINIKUBE_IP iicpc.local/" /etc/hosts
  log_success "Updated iicpc.local → $MINIKUBE_IP"
else
  echo "$MINIKUBE_IP iicpc.local" | sudo tee -a /etc/hosts >/dev/null
  log_success "Added iicpc.local → $MINIKUBE_IP"
fi

# -------------------------------------------------------
# FULL WIPE (only with --full)
# -------------------------------------------------------
if [ "$FULL_DEPLOY" = true ]; then
  log_step "Full Wipe"

  log_warn "Deleting dynamic sandbox/kaniko/bot-fleet resources..."
  kubectl delete pods -n "$NAMESPACE" -l app=sandbox   --ignore-not-found=true
  kubectl delete svc  -n "$NAMESPACE" -l app=sandbox   --ignore-not-found=true
  kubectl delete jobs -n "$NAMESPACE" -l app=kaniko    --ignore-not-found=true
  kubectl delete jobs -n "$NAMESPACE" -l app=bot-fleet --ignore-not-found=true

  log_warn "Uninstalling Helm release..."
  helm uninstall "$PLATFORM_NAME" -n "$NAMESPACE" 2>/dev/null \
    && log_success "Helm release removed" \
    || log_warn "No existing Helm release to remove"

  log_warn "Removing all minikube cached images for this project..."
  for NAME in "${!SERVICE_PATHS[@]}"; do
    minikube image ls 2>/dev/null \
      | grep "^${NAME}:" \
      | xargs -r -I{} minikube image rm {} 2>/dev/null \
      && log_info "  Removed cached images for: $NAME" || true
  done

  if kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; then
    log_warn "Namespace '$NAMESPACE' found — deleting..."
    kubectl delete namespace "$NAMESPACE" 2>/dev/null || true
  else
    log_warn "Namespace '$NAMESPACE' not found — skipping delete"
  fi
  log_info "Waiting for namespace to fully terminate..."
  while kubectl get namespace "$NAMESPACE" >/dev/null 2>&1; do
    sleep 2
  done
  log_success "Namespace fully terminated"

  log_success "Full wipe complete"
fi

# -------------------------------------------------------
# NAMESPACE
#
# MUST be after full wipe — helm uninstall deletes the
# namespace too, so we recreate it here unconditionally.
# kubectl apply is idempotent: works whether namespace
# exists or was just deleted. This is why --create-namespace
# on the helm command is not needed.
# -------------------------------------------------------
log_step "Namespace"

kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -
log_success "Namespace '$NAMESPACE' ready"

# -------------------------------------------------------
# DOCKER REGISTRY SECRET
#
# Lives here — AFTER namespace creation — so it works
# correctly for both --full (fresh) and incremental runs.
#
# HOW TO SET YOUR CREDENTIALS before running:
#   export DOCKER_USERNAME="your_dockerhub_username"
#   export DOCKER_PASSWORD='your_access_token'
#                           ↑ single quotes — password may contain special chars
#
# Get a token (safer than password):
#   https://hub.docker.com/settings/security → New Access Token
# -------------------------------------------------------
log_step "Docker Registry Secret"

if [ -n "$DOCKER_USERNAME" ] && [ -n "$DOCKER_PASSWORD" ]; then
  kubectl create secret docker-registry dockerhub-secret \
    --namespace="$NAMESPACE" \
    --docker-server=https://index.docker.io/v1/ \
    --docker-username="$DOCKER_USERNAME" \
    --docker-password="$DOCKER_PASSWORD" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  log_success "Docker registry secret applied (user: $DOCKER_USERNAME)"
else
  kubectl create secret docker-registry dockerhub-secret \
    --namespace="$NAMESPACE" \
    --docker-server=https://index.docker.io/v1/ \
    --docker-username="placeholder" \
    --docker-password="placeholder" \
    --dry-run=client -o yaml | kubectl apply -f - >/dev/null
  log_success "Docker registry secret created (no credentials — internal registry only)"
fi

# -------------------------------------------------------
# BUILD + LOAD IMAGES
#
# WHY NOT :latest —
#   imagePullPolicy: Never (your values.yaml) means Kubernetes
#   uses whatever is in minikube's local cache.
#   If the tag is :latest and it already exists in cache,
#   Kubernetes uses the OLD image even after rollout restart.
#   A unique tag (git SHA + timestamp) forces Kubernetes to
#   treat it as a genuinely new image every time.
#   Works correctly for judges cloning fresh too — their
#   machine has never seen this tag so it must use the build.
# -------------------------------------------------------
log_step "Building Docker Images (tag: $BUILD_TAG)"

build_and_load() {
  local NAME="$1"
  local PATH_="$2"

  log_info "Building $NAME:$BUILD_TAG from $PATH_..."
  docker build --no-cache -t "${NAME}:${BUILD_TAG}" "$PATH_"
  log_success "$NAME built"

  log_info "Loading $NAME:$BUILD_TAG into minikube..."
  minikube image load "${NAME}:${BUILD_TAG}"
  log_success "$NAME loaded into minikube"
}

if [ -n "$SINGLE_SERVICE" ]; then
  PATH_="${SERVICE_PATHS[$SINGLE_SERVICE]}"
  [ -z "$PATH_" ] && log_error "Unknown service: $SINGLE_SERVICE. Valid: ${!SERVICE_PATHS[*]}"
  build_and_load "$SINGLE_SERVICE" "$PATH_"
else
  for NAME in "${!SERVICE_PATHS[@]}"; do
    build_and_load "$NAME" "${SERVICE_PATHS[$NAME]}"
  done
fi

# -------------------------------------------------------
# HELM DEPLOY
#
# --set global.imageTag overrides the 'latest' in values.yaml
# with our unique build tag. This is how the unique tag
# reaches your deployment templates.
#
# Your Helm ingress.yaml uses {{ .Values.global.namespace }}
# which is 'iicpc' — the namespace was already created above
# with kubectl apply, so Helm finds it waiting and happy.
# -------------------------------------------------------
if [ -z "$SINGLE_SERVICE" ]; then
log_step "Helm Deploy"

kubectl label namespace "$NAMESPACE" app.kubernetes.io/managed-by=Helm --overwrite
kubectl annotate namespace "$NAMESPACE" meta.helm.sh/release-name="$PLATFORM_NAME" --overwrite
kubectl annotate namespace "$NAMESPACE" meta.helm.sh/release-namespace="$NAMESPACE" --overwrite

REGISTRY_IP=$(kubectl get svc registry -n kube-system -o jsonpath='{.spec.clusterIP}')
log_info "Registry IP: ${REGISTRY_IP}"

helm upgrade --install "$PLATFORM_NAME" "$HELM_CHART" \
  --namespace "$NAMESPACE" \
  --set global.imageTag="${BUILD_TAG}" \
  --set global.correctnessHarnessImageTag="${BUILD_TAG}" \
  --set submissionService.registryAddress="${REGISTRY_IP}:80" \
  --set submissionService.registryMirror="${REGISTRY_IP}:80" \
  --wait \
  --timeout 120s

log_success "Helm deploy complete (imageTag: $BUILD_TAG)"
fi

# -------------------------------------------------------
# ROLLOUT
# -------------------------------------------------------
if [ -n "$SINGLE_SERVICE" ]; then
  log_info "Updating image for $SINGLE_SERVICE..."

  if [ "$SINGLE_SERVICE" = "bot-fleet" ]; then
    kubectl set env deployment/submission-service \
      -n "$NAMESPACE" \
      BOT_FLEET_IMAGE_TAG="$BUILD_TAG"
    kubectl rollout status deployment/submission-service -n "$NAMESPACE" --timeout=90s

  elif [ "$SINGLE_SERVICE" = "correctness-harness" ]; then
    kubectl set env deployment/submission-service \
      -n "$NAMESPACE" \
      CORRECTNESS_HARNESS_IMAGE_TAG="$BUILD_TAG"
    kubectl rollout status deployment/submission-service -n "$NAMESPACE" --timeout=90s

  else
    kubectl set image deployment/"$SINGLE_SERVICE" \
      "${SINGLE_SERVICE}=${SINGLE_SERVICE}:${BUILD_TAG}" \
      -n "$NAMESPACE"
    kubectl rollout status deployment/"$SINGLE_SERVICE" -n "$NAMESPACE" --timeout=90s
  fi

else
  log_step "Rolling Restart (all deployments)"

  # Clean up dynamic resources before rollout
  log_info "Cleaning up sandbox/kaniko/bot-fleet resources..."
  kubectl delete pods -n "$NAMESPACE" -l app=sandbox   --ignore-not-found=true
  kubectl delete svc  -n "$NAMESPACE" -l app=sandbox   --ignore-not-found=true
  kubectl delete jobs -n "$NAMESPACE" -l app=kaniko    --ignore-not-found=true
  kubectl delete jobs -n "$NAMESPACE" -l app=bot-fleet --ignore-not-found=true

  kubectl rollout restart deployment -n "$NAMESPACE"
  kubectl rollout status  deployment -n "$NAMESPACE" --timeout=120s
  log_success "All deployments rolled out"
fi

# -------------------------------------------------------
# WAIT FOR INGRESS
# -------------------------------------------------------
log_step "Waiting for Ingress"
sleep 8
log_success "Ingress ready"

# -------------------------------------------------------
# DONE
# -------------------------------------------------------
echo ""
echo "=============================================="
echo -e "  ${GREEN}Deploy Complete!${NC}"
echo "  Build tag: $BUILD_TAG"
echo "=============================================="
echo ""
echo "  Platform     : http://iicpc.local"
echo "  Submit       : http://iicpc.local/submit"
echo "  Leaderboard  : http://iicpc.local/scores"
echo ""
echo "  ── Status commands ──────────────────────────"
echo "  All resources : kubectl get all -n iicpc"
echo "  Pods only     : kubectl get pods -n iicpc"
echo "  Ingress       : kubectl get ingress -n iicpc"
echo "  Events        : kubectl get events -n iicpc --sort-by=.lastTimestamp"
echo ""
echo "  ── Logs ─────────────────────────────────────"
echo "  kubectl logs -f deployment/submission-service -n iicpc"
echo "  kubectl logs -f deployment/leaderboard-service -n iicpc"
echo ""
echo "  ── Dashboard ────────────────────────────────"
echo "  minikube dashboard"
echo ""
echo "  ── Frontend ─────────────────────────────────"
echo "  cd frontend && npm run dev"
echo "=============================================="
echo ""

# export DOCKER_USERNAME=yourusername
# export DOCKER_PASSWORD=yourtoken
# stapankumar
# 7U;ZX=*y9U3sA!y