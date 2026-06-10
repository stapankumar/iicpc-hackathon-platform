# Cloud Deployment Guide

This guide covers deploying the IICPC platform on AWS, GCP, and Azure for real contest use with 100+ participants. The platform code is identical — only the Kubernetes cluster and ingress configuration changes.

---

## Prerequisites (all providers)

```bash
docker
kubectl
helm
git
```

Your machine needs access to the cloud cluster via `kubectl`.

---

## AWS — EKS

### 1. Create cluster

```bash
eksctl create cluster \
  --name iicpc-platform \
  --region ap-south-1 \
  --nodegroup-name workers \
  --node-type t3.xlarge \
  --nodes 3 \
  --nodes-min 2 \
  --nodes-max 10 \
  --managed
```

Minimum node type: `t3.xlarge` (4 vCPU, 16GB RAM). Use `t3.2xlarge` for 50+ concurrent submissions.

### 2. Install ingress controller

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/aws/deploy.yaml
```

### 3. Get public URL

```bash
kubectl get svc ingress-nginx-controller -n ingress-nginx
# Note the EXTERNAL-IP — this is your contest URL
```

### 4. Deploy platform

```bash
# No minikube needed — skip that section in deploy.sh
# Build images, push to ECR or Docker Hub, then:
helm upgrade --install iicpc-platform helm/iicpc-platform \
  --namespace iicpc \
  --create-namespace \
  --set global.imageTag=<your-tag> \
  --set ingress.host=<your-external-ip-or-domain>
```

### 5. Share with contestants

```
http://<EXTERNAL-IP>/submit
http://<EXTERNAL-IP>          ← leaderboard
```

---

## GCP — GKE

### 1. Create cluster

```bash
gcloud container clusters create iicpc-platform \
  --zone asia-south1-a \
  --machine-type e2-standard-4 \
  --num-nodes 3 \
  --enable-autoscaling \
  --min-nodes 2 \
  --max-nodes 10
```

### 2. Get credentials

```bash
gcloud container clusters get-credentials iicpc-platform --zone asia-south1-a
```

### 3. Install ingress controller

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
```

### 4. Get public URL

```bash
kubectl get svc ingress-nginx-controller -n ingress-nginx
# Note the EXTERNAL-IP
```

### 5. Deploy platform

```bash
helm upgrade --install iicpc-platform helm/iicpc-platform \
  --namespace iicpc \
  --create-namespace \
  --set global.imageTag=<your-tag> \
  --set ingress.host=<your-external-ip-or-domain>
```

---

## Azure — AKS

### 1. Create resource group and cluster

```bash
az group create --name iicpc-rg --location eastus

az aks create \
  --resource-group iicpc-rg \
  --name iicpc-platform \
  --node-count 3 \
  --node-vm-size Standard_D4s_v3 \
  --enable-cluster-autoscaler \
  --min-count 2 \
  --max-count 10 \
  --generate-ssh-keys
```

### 2. Get credentials

```bash
az aks get-credentials --resource-group iicpc-rg --name iicpc-platform
```

### 3. Install ingress controller

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.8.2/deploy/static/provider/cloud/deploy.yaml
```

### 4. Get public URL

```bash
kubectl get svc ingress-nginx-controller -n ingress-nginx
```

### 5. Deploy platform

```bash
helm upgrade --install iicpc-platform helm/iicpc-platform \
  --namespace iicpc \
  --create-namespace \
  --set global.imageTag=<your-tag> \
  --set ingress.host=<your-external-ip-or-domain>
```

---

## Capacity Planning

| Participants | Node type | Node count |
|---|---|---|
| Up to 50 | t3.xlarge / e2-standard-4 | 3 |
| Up to 200 | t3.2xlarge / e2-standard-8 | 5 |
| Up to 500 | t3.2xlarge / e2-standard-8 | 10 |
| 1000+ | c5.4xlarge / c2-standard-16 | 15+ |

Each concurrent submission uses ~5 CPU cores and ~1.7GB RAM during judging. Submissions are queued via Redis — the platform handles concurrent load automatically.

---

## Image Registry

For cloud deployment, build and push images to a registry accessible by your cluster:

**Docker Hub:**
```bash
docker build -t yourusername/submission-service:tag ./backend/submission-service
docker push yourusername/submission-service:tag
```

**AWS ECR / GCP Artifact Registry / Azure ACR** — follow your cloud provider's container registry docs.

Update `helm/iicpc-platform/values.yaml` image references accordingly.

---

## Resetting Between Contest Rounds

Same as local — see judges-guide.md Resetting Between Rounds section.

---

## Teardown

**AWS:**
```bash
eksctl delete cluster --name iicpc-platform --region ap-south-1
```

**GCP:**
```bash
gcloud container clusters delete iicpc-platform --zone asia-south1-a
```

**Azure:**
```bash
az group delete --name iicpc-rg --yes
```