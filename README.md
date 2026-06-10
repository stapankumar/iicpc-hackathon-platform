# IICPC Distributed Benchmarking Platform

A production-grade distributed platform for evaluating contestant-submitted
orderbook implementations under simulated peak market conditions.

Built for the IICPC Summer Hackathon 2026.

---

## What This Platform Does

Contestants submit their orderbook server code. The platform:

1. Containerizes and deploys their code in an isolated K8s sandbox
2. Spawns a fleet of 100+ concurrent trading bots that hammer their endpoints
3. Measures p50/p90/p99 latency, TPS, and fill correctness in real time
4. Ranks all contestants live on a streaming leaderboard

Think Codeforces — but for quantitative trading infrastructure.

---

## System Requirements

| Requirement | Minimum | Recommended |
|---|---|---|
| OS | Ubuntu 20.04+ | Ubuntu 24.04 |
| RAM | 8GB | 16GB |
| CPU | 4 cores | 8+ cores |
| Disk | 20GB free | 40GB free |
| Go | 1.24.0 | 1.24.0 |
| Node.js | 18+ | 22+ |
| Docker | 20+ | 26+ |
| minikube | 1.38.1 | 1.38.1 |
| kubectl | 1.28+ | 1.30+ |
| Helm | 3.0+ | 3.21+ |

---

## Installation

### 1. Install Go 1.24.0
```bash
sudo rm -rf /usr/local/go
wget https://go.dev/dl/go1.24.0.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.24.0.linux-amd64.tar.gz
echo 'export PATH=$PATH:/usr/local/go/bin' >> ~/.bashrc
source ~/.bashrc
go version  # should show go1.24.0
```

### 2. Install minikube
```bash
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube
minikube version
```

### 3. Install Helm
```bash
curl https://raw.githubusercontent.com/helm/helm/main/scripts/get-helm-3 | bash
helm version
```

### 4. Clone the repository
```bash
git clone https://github.com/iicpc/hackathon-platform.git
cd hackathon-platform
```

---

## Running the Platform

### Option A — Local Development (recommended for testing)

**One-time setup:**
```bash
# Create local frontend config (only needed once)
cat > frontend/.env.local << 'EOF'
VITE_LEADERBOARD_URL=http://localhost:8082
VITE_SUBMISSION_URL=http://localhost:8081
EOF
```

Make sure Redis is running locally:
```bash
redis-server --daemonize yes
```

**Before each fresh test run:**
```bash
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL leaderboard
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL leaderboard:details
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL telemetry:orders
```

Run each service in a separate terminal:

**Terminal 1 — Mock Exchange (fake contestant server)**
```bash
cd mock-exchange
go run main.go
# Running on :8080
```

**Terminal 2 — Telemetry Service**
```bash
cd backend/telemetry-service
go run main.go
# Listening on Redis Stream: telemetry:orders
```

**Terminal 3 — Leaderboard Service**
```bash
cd backend/leaderboard-service
go run main.go
# Running on :8082
```

**Terminal 4 — Frontend**
```bash
cd frontend
npm install
npm run dev
# http://localhost:5173
```

**Terminal 5 — Run Bot Fleet (triggers load test)**
```bash
cd backend/bot-fleet
go run main.go
# Launches 100 bots → http://localhost:8080
# Done! 5000 orders in 411ms → 12158 orders/sec
```

---

### Option B — Kubernetes Deployment via Helm

**Prerequisites** — install these once:
- [Docker](https://docs.docker.com/get-docker/)
- [minikube](https://minikube.sigs.k8s.io/docs/start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm](https://helm.sh/docs/intro/install/)
- [Go 1.24.0](https://go.dev/dl/)
- [Node.js 18+](https://nodejs.org/)

**Deploy:**
```bash
git clone <repo-url>
cd iicpc-hackathon-platform

# Set Docker Hub credentials to avoid rate limits (required)
export DOCKER_USERNAME=your_dockerhub_username
export DOCKER_PASSWORD=your_access_token  # hub.docker.com/settings/security → New Access Token

./deploy.sh --full
```

The script automatically:
- Detects your system CPU/RAM and allocates 60% to minikube
- Enables Ingress and metrics-server addons
- Configures `/etc/hosts` for `iicpc.local`
- Builds all 7 Docker images (including correctness harness)
- Loads images into minikube
- Deploys entire platform via Helm
- Rolls out all services with zero downtime

**Start frontend:**
```bash
cd frontend
npm install
npm run dev
```

**Open browser:**
http://localhost:5173

> Note: `deploy.sh` will ask for sudo password once to update `/etc/hosts`. This is the only manual step — required to map `iicpc.local` to your minikube cluster IP.

**To start fresh (clear all scores):**
```bash
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL leaderboard
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL leaderboard:details
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL telemetry:orders
kubectl rollout restart deployment/telemetry-service -n iicpc
```

---

## How to Submit (For Contestants)

### What to submit
A `.zip` file containing:
```bash
your-submission.zip
├── Dockerfile          ← required
├── main.go             ← your orderbook implementation
└── (any other files)
```

### Your server MUST expose these endpoints on port 8080

**Place an order:**
```bash
POST /order
Content-Type: application/json

{
  "side": "buy",
  "type": "limit",
  "price": 99.5,
  "quantity": 10
}

Response:
{
  "order_id": "uuid",
  "status": "ACK",
  "filled_qty": 6,
  "timestamp": 1234567890
}
```

**Cancel an order:**
```bash
DELETE /order?order_id=<uuid>

Response:
{
  "order_id": "uuid",
  "status": "CANCELLED"
}
```

**Get orderbook state:**
```bash
GET /orderbook

Response:
{
  "bids": [{"price": 99.5, "quantity": 100}],
  "asks":  [{"price": 100.5, "quantity": 150}]
}
```

### Scoring formula
```
Score = (TPS × 0.4) + (1/p99_ms × 1000 × 0.4) + (correctness% × 0.2)
```

---

## Architecture Overview
```text
Contestant Upload (zip + team name)
       │
       ▼
Submission Service (:8081)
       │
       ├─ Step 1: Kaniko Job — builds Docker image from zip
       │
       ├─ Step 2: Sandbox Pod — runs contestant's orderbook on :8080
       │
       ├─ Step 3: Correctness Harness Job — 6 sequential scenarios
       │          publishes correctness score to Redis
       │
       └─ Step 4: Bot Fleet Job — 500 bots × 100 orders = 50,000 orders
                  publishes latencies to Redis Stream
                  sends done signal when complete
                         │
                         ▼
              Redis Streams (telemetry:orders)
                         │
                         ▼
              Telemetry Service
                • p50 / p90 / p99 latency
                • real TPS (elapsed time)
                • correctness from Redis
                • composite score → leaderboard
                • sandbox cleanup after scoring
                         │
                         ▼
              Redis Sorted Set (leaderboard)
              Redis Hash (leaderboard:details)
                         │
                         ▼
              Leaderboard Service (:8082)
                • SSE stream every 2s
                         │
                         ▼
              Frontend (:5173)
                • live leaderboard
                • submission portal
                • pipeline progress tracker
```

---

## Tech Stack & Justifications

| Component | Technology | Justification |
|---|---|---|
| All services | Go 1.24 | M:N goroutine scheduler, no GIL, 12,000+ orders/sec on laptop |
| Message bus | Redis Streams | Producer-consumer decoupling, prevents telemetry bottleneck polluting measurements |
| Orchestration | Kubernetes | Native Job isolation, ResourceQuota, NetworkPolicy per submission |
| Sandbox isolation | K8s Jobs + SecurityContext | CPU pinning, memory limits, read-only filesystem, non-root execution |
| Bot fleet scaling | K8s HPA | Scales 2→10 pods automatically on CPU pressure |
| Frontend | React + Vite + Tailwind | SSE for live updates, zero-config build |

---

## Project Structure
```text
iicpc-hackathon-platform/
├── mock-exchange/          # Fake contestant server for testing
├── backend/
│   ├── submission-service/  # Receives uploads, spawns K8s pipeline
│   ├── bot-fleet/           # 500 concurrent trading bots
│   ├── correctness-harness/ # 6 sequential correctness scenarios
│   ├── telemetry-service/   # Scoring, leaderboard, sandbox cleanup
│   └── leaderboard-service/ # Score API + SSE streaming
├── frontend/               # React leaderboard UI
├── k8s/                    # Kubernetes manifests
├── helm/                   # Helm chart for parameterized deploy
└── docs/                   # Architecture blueprint and API specs
```

---

## Performance Benchmarks (Measured on HP ProBook, Ryzen, 14GB RAM)

| Metric | Value |
|---|---|
| Bot fleet throughput | 12,158 orders/sec |
| Mock exchange p50 | 9ms |
| Mock exchange p90 | 17ms |
| Mock exchange p99 | 27ms |
| Concurrent bots | 100 goroutines |
| K8s pod startup | ~3 seconds |

---

## Team
IICPC Summer Hackathon 2026