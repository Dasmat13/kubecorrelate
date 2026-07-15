# KubeCorrelate

[![Go Version](https://img.shields.io/github/go-mod/go-version/Dasmat13/kubecorrelate)](https://golang.org)
[![License](https://img.shields.io/github/license/Dasmat13/kubecorrelate)](LICENSE)

**KubeCorrelate** (`kubecorrelate`) is a lightweight, high-utility CLI tool built in Go that simplifies Kubernetes microservice debugging. It merges application container logs, Kubernetes events, configuration shifts, and underlying node-level resource warnings into a single, time-aligned, color-coded stream.

No more switching between multiple terminal windows running `stern`, `kubectl get events -w`, and checking node conditions manually. KubeCorrelate stitches them together on your terminal in real time.

---

## 💡 Why KubeCorrelate?

* **Chronological Timeline:** Log lines, liveness probe failures, ConfigMap updates, and OOMKills are printed sequentially down to the millisecond.
* **Bounded Slop Buffer:** Employs an artificial delay (1.5 seconds) to buffer and sort out-of-order streams caused by API aggregation and network delays before printing.
* **Graceful Degradation:** Safely runs under limited developer RBAC credentials. If Node-level or Secret-level watches are unauthorized, the tool warns you once and degrades gracefully instead of crashing.
* **API-Level Timestamps:** Force-enables Kubernetes log timestamps internally and automatically strips the header so your output stays clean, regardless of your application's formatting structure.

---

## 🏗️ How it Works

```
                     +---------------------------------------+
                     | KubeCorrelate Command-Line Invocation |
                     +-------------------+-------------------+
                                         |
                       +-----------------+-----------------+
                       |                                   |
           +-----------v-----------+           +-----------v-----------+
           |  Log Streaming Loop   |           |  Event Informer Loop  |
           +-----------+-----------+           +-----------+-----------+
                       |                                   |
                       +-----------------+-----------------+
                                         |
                       +-----------------+-----------------+
                       |                                   |
           +-----------v-----------+           +-----------v-----------+
           |   ConfigMap/Secrets   |           | Node Pressure Watcher |
           +-----------+-----------+           +-----------+-----------+
                       |                                   |
                       +-----------------+-----------------+
                                         |
                       +-----------------v-----------------+
                       |   Bounded Slop Sorting Buffer     |
                       +-----------------+-----------------+
                                         |
                       +-----------------v-----------------+
                       |      Unified Colorized TUI        |
                       +-----------------------------------+
```

---

## 🚀 Installation

```bash
# Clone the repository
git clone https://github.com/Dasmat13/kubecorrelate.git
cd kubecorrelate

# Compile the binary
go build -o bin/kubecorrelate cmd/kubecorrelate/main.go
```

---

## 📖 Usage

Run it against target pods by specifying a namespace and label selector (uses your active `kubeconfig` context by default):

```bash
# Monitor default namespace using a label selector
./bin/kubecorrelate -l app=order-processor

# Monitor all namespaces
./bin/kubecorrelate -A

# Watch pods matching a specific name regex
./bin/kubecorrelate -n staging -p "^api-gateway-[a-z0-9]+$"

# Adjust logs starting lookback window (default is 10m)
./bin/kubecorrelate -l app=auth-service --since 1h
```

---

## 🎨 Log Stream Output Example

Here is how KubeCorrelate displays a unified debugging timeline when a database configuration changes right before an Out-Of-Memory (OOM) event occurs:

```text
[23:55:01.002] [LOG] [order-processor-7fd8b/processor] Processing order #98213...
[23:55:02.155] [CONFIG] [processor-routing-rules] ConfigMap "processor-routing-rules" was modified (resourceVersion: 1084)
[23:55:03.441] [LOG] [order-processor-7fd8b/processor] Reloading routing rules...
[23:55:05.801] [EVENT] [order-processor-7fd8b] [WARN] LivenessProbeFailed: Liveness probe failed (HTTP 500)
[23:55:05.912] [LOG] [order-processor-7fd8b/processor] Critical: Database connection timeout!
[23:55:07.100] [NODE] [gke-node-pool-1a] [WARN] MemoryPressure: System memory usage exceeded 95%
[23:55:07.350] [EVENT] [order-processor-7fd8b] [WARN] BackOff: Container restarted (Reason: OOMKilled, Exit Code 137)
[23:55:10.021] [LOG] [order-processor-7fd8b/processor] Starting Order Processor v1.4.2...
```

---

## 🛠️ Running Tests

To verify both the timestamp parsing and bounded slop buffer sorting modules, run:

```bash
go test -v ./...
```

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.
