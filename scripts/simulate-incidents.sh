#!/usr/bin/env bash

set -euo pipefail

NAMESPACE="kubecorrelate-demo"

echo "=== 🚀 Initializing KubeCorrelate Incident Simulation ==="
echo "Creating namespace: ${NAMESPACE}"
kubectl create namespace "${NAMESPACE}" --dry-run=client -o yaml | kubectl apply -f -

# 1. CrashLoopBackOff Simulation
echo "Creating CrashLoopBackOff pod..."
cat <<EOF | kubectl apply -n "${NAMESPACE}" -f -
apiVersion: v1
kind: Pod
metadata:
  name: crashloop-demo
  labels:
    app: crashloop-demo
spec:
  containers:
  - name: alpine-crasher
    image: alpine:latest
    command: ["/bin/sh", "-c"]
    args:
    - |
      echo "[$(date +%T)] Starting processing job..."
      sleep 2
      echo "[$(date +%T)] Encountered fatal segmentation fault! Exiting..."
      exit 1
EOF

# 2. OOMKilled Simulation
echo "Creating OOMKilled pod..."
cat <<EOF | kubectl apply -n "${NAMESPACE}" -f -
apiVersion: v1
kind: Pod
metadata:
  name: oomkilled-demo
  labels:
    app: oomkilled-demo
spec:
  containers:
  - name: memory-hog
    image: alpine:latest
    command: ["/bin/sh", "-c"]
    args:
    - |
      echo "[$(date +%T)] Booting memory-intensive service..."
      # Allocate memory using a shell loop allocating env vars or big strings
      # till container is OOMKilled by kernel.
      stress_str="a"
      while true; do
        stress_str="\$stress_str\$stress_str\$stress_str"
        sleep 0.5
      done
    resources:
      limits:
        memory: "16Mi"
EOF

# 3. Failed Rollout Simulation
echo "Creating Deployment for Failed Rollout simulation..."
cat <<EOF | kubectl apply -n "${NAMESPACE}" -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rollout-demo
spec:
  replicas: 2
  selector:
    matchLabels:
      app: rollout-demo
  template:
    metadata:
      labels:
        app: rollout-demo
    spec:
      containers:
      - name: main-app
        image: nginx:latest
        ports:
        - containerPort: 80
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: rollout-demo
spec:
  template:
    spec:
      containers:
      - name: main-app
        image: nginx:does-not-exist-tag
EOF

echo ""
echo "=== 🎉 Incidents Initialized ==="
echo "To see KubeCorrelate in action, run:"
echo "  kubecorrelate -n ${NAMESPACE} --since 5m"
echo ""
echo "To clean up the environment, run:"
echo "  kubectl delete namespace ${NAMESPACE}"
