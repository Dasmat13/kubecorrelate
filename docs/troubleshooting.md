# KubeCorrelate Troubleshooting Guide

This guide details common issues you might run into when using KubeCorrelate, and how to resolve them.

---

## 🔒 RBAC Permission Warnings
When running KubeCorrelate, you may see warnings like:
```text
⚠️ Node pressure monitoring disabled due to insufficient cluster-wide RBAC permissions
⚠️ Log monitoring disabled in namespace "default" due to insufficient RBAC permissions
```
### Why this happens
KubeCorrelate watches four different types of resources to correlate telemetry: Pods/Logs, Events, ConfigMaps/Secrets, and Nodes. Watching nodes, configmaps, or namespaces requires specific RBAC roles. If your active service account or kubeconfig does not have read/watch access to those, KubeCorrelate will warn you and gracefully degrade, falling back to only monitoring the resources you do have access to.

### How to fix it
Ensure your Kubernetes role/clusterrole contains rules for watching the restricted resources:
```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: kubecorrelate-reader
rules:
  - apiGroups: [""]
    resources: ["pods", "pods/log", "events", "configmaps", "secrets"]
    verbs: ["get", "list", "watch"]
  - apiGroups: [""]
    resources: ["nodes"]
    verbs: ["get", "list", "watch"]
```

---

## ⏳ Out-of-Order Logs or Events
If you notice that some logs or events appear out of chronological order on the terminal:
*   **Adjust `--buffer-delay`:** By default, KubeCorrelate buffers incoming events for `1.5s` to sort them chronologically (accounting for network latency from the Kubernetes API server). If you are running against a high-latency cluster (e.g. multi-region or under heavy congestion), increase this duration:
    ```bash
    kubecorrelate -buffer-delay 3s
    ```

---

## 🌐 API Server Connection Drops / Reconnect Loops
If KubeCorrelate keeps logging error events or warnings about watcher restarts:
*   Verify your network connectivity to the API server: `kubectl cluster-info`
*   If your token has expired, re-authenticate to update your kubeconfig context.
*   By default, KubeCorrelate automatically retries connection drops with exponential backoff.
