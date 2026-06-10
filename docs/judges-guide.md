# IICPC Summer Hackathon 2026 — Judges Operations Guide

## Prerequisites

Install on the judging machine:

```bash
# Required
docker
minikube
kubectl  
helm
git
```

Minimum specs: 8GB RAM, 4 CPU cores, 20GB free disk.

---

## One-Time Setup

Set Docker Hub credentials to avoid rate limits:

```bash
export DOCKER_USERNAME=your_dockerhub_username
export DOCKER_PASSWORD=your_access_token
```

Get a token at: https://hub.docker.com/settings/security → New Access Token
```bash
git clone <repo-url>
cd iicpc-hackathon-platform
./deploy.sh --full
```

This takes 5-10 minutes on first run. It will:
- Start minikube
- Build all service images
- Deploy everything via Helm
- Configure `/etc/hosts` for `iicpc.local`

When complete you will see:
```
[OK] Platform ready at http://iicpc.local
```

---

## During the Contest

**Submission portal** — share this URL with contestants:
```
http://iicpc.local/submit  (if on same network)
```

For contestants on different machines, replace `iicpc.local` with the judge machine's local IP:
```bash
hostname -I | awk '{print $1}'
# Use that IP instead of iicpc.local
```

**Live leaderboard** — open in browser:
```
http://iicpc.local
```

Leaderboard updates automatically via SSE — no refresh needed.

---

## Monitoring During Judging

Watch pipeline progress as submissions come in:

```bash
# See all jobs running
kubectl get pods -n iicpc -w

# Watch scoring happen live
kubectl logs -f deployment/telemetry-service -n iicpc

# Check a specific submission status
curl "http://iicpc.local/status?submission_id=<id>"
```

---

## Understanding the Leaderboard

| Column | Meaning |
|---|---|
| Team | Team name entered at submission |
| p99 (ms) | Worst-case latency — lower is better |
| TPS | Orders processed per second — higher is better |
| Correctness | % of correctness scenarios passed — higher is better |
| Score | Composite: 40% TPS + 40% latency + 20% correctness |
| Attempts | How many times this team submitted |

Score shown is always the team's **best** score across all attempts.

---

## Correctness Scenarios

Each submission is tested against 6 scenarios before load testing:

| Scenario | Tests |
|---|---|
| S1 | Basic price cross — buy at 105 fills against sell at 100 |
| S2 | No cross — buy at 90 does not fill against sell at 95 |
| S3 | Cancel prevents fill — cancelled order never matches |
| S4 | Partial fill — buy 10 fills 3 against resting sell of 3 |
| S5 | Market order fills immediately against resting liquidity |
| S6 | GET /orderbook returns valid bids/asks structure |

Score = scenarios passed / 6. Shown as percentage on leaderboard.

---

## Resetting Between Rounds

To clear all scores and start fresh:

```bash
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL leaderboard
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL leaderboard:details
kubectl exec -it redis-0 -n iicpc -- redis-cli DEL telemetry:orders
kubectl rollout restart deployment/telemetry-service -n iicpc
```

Do NOT use `FLUSHALL` — it wipes the Redis stream and breaks telemetry until restart.

---

## Teardown After Contest

```bash
./uninstall.sh
```

This removes all Kubernetes resources, stops minikube, and cleans up the hosts file entry.

---

## Cloud Deployment

For real contest use with 100+ participants, deploy on a cloud Kubernetes cluster instead of minikube.

See [cloud-deployment.md](./cloud-deployment.md) for step-by-step guides for:
- AWS EKS
- GCP GKE  
- Azure AKS