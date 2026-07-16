# Stop Switching Between 5 Different Kubectl Commands During an Outage

Every Kubernetes engineer knows the drill. Your phone buzzes. PagerDuty is firing. A critical microservice is failing in production. 

You open your terminal and begin the standard "incident dance":
1. `kubectl get pods -n production` (to see what is failing).
2. `kubectl logs -f deployment/payment-service -n production --all-containers` (to watch the logs).
3. `kubectl get events -n production -w` in another tab (to track lifecycle warnings).
4. `kubectl describe pod/payment-service-xyz -n production` in a third tab (to check liveness/readiness probe failures).
5. `kubectl top nodes` in a fourth tab (to see if the underlying node is under resource pressure).

You are constantly context-switching, manually aligning timestamps, and trying to build a mental timeline of the failure in your head. 

**Is this log line from before or after the container was OOMKilled? Why did the liveness probe fail right when the database pool saturated?**

This is why we built **KubeCorrelate** (`kubectl correlate`).

---

## The Solution: A Chronological Telemetry Stream

**KubeCorrelate** is a zero-dependency, lightweight CLI utility that connects directly to your active Kubernetes context. It acts as a client-side telemetry multiplexer, combining four critical streams into a single, time-aligned terminal stream in real time:

1. **Pod Logs**: Standard output and error streams from all matching container replicas.
2. **API Events**: Warnings, lifecycle states, image pulls, and mounting states.
3. **Configuration Shifts**: Dynamic config changes and controller updates.
4. **Node Pressures**: Sub-system alerts (MemoryPressure, DiskPressure) from the host node.

Instead of hunting across multiple windows, you run a single command and see the exact chronological root cause down to the millisecond.

---

## Real-World Scenario: Debugging a CrashLoopBackOff

Let's look at how KubeCorrelate visualizes a classic `CrashLoopBackOff` incident:

```text
2026-07-16T15:21:00Z [EVENT] [pod/auth-service-7f8d] Scheduled to node-worker-1
2026-07-16T15:21:01Z [EVENT] [pod/auth-service-7f8d] Pulling image "auth:v1.2.0"
2026-07-16T15:21:03Z [EVENT] [pod/auth-service-7f8d] Successfully pulled image
2026-07-16T15:21:04Z [LOG]   [pod/auth-service-7f8d] [main] Starting auth service...
2026-07-16T15:21:05Z [LOG]   [pod/auth-service-7f8d] [db] Connecting to database master...
2026-07-16T15:21:08Z [LOG]   [pod/auth-service-7f8d] [db] Connection timeout!
2026-07-16T15:21:09Z [EVENT] [pod/auth-service-7f8d] Liveness probe failed: HTTP probe failed with statuscode: 500
2026-07-16T15:21:10Z [EVENT] [pod/auth-service-7f8d] Killing container "main-app"
```

Notice how the database connection timeout log is immediately followed by the liveness probe failure event. You don't need to manually cross-reference timestamps—the timeline speaks for itself.

---

## Under the Hood: Bounded Slop Sorting

Why hasn't this been done cleanly before? If you simply print events as they arrive, network latency and API server delay will cause logs and events to arrive out of order. 

KubeCorrelate solves this with a client-side **Bounded Slop Sorting Buffer** (configurable via `--buffer-delay`, defaulting to `1.5s`). It buffers incoming signals for a tiny fraction of a second, sorts them by their native RFC3339 timestamps, and flushes them in absolute chronological order. 

The delay is barely noticeable to a human, but it guarantees a perfectly accurate timeline.

---

## 2-Minute Quick Start

### 1. Installation
Install KubeCorrelate via Krew in one command:
```bash
kubectl krew install correlate
```

### 2. Basic Usage
Monitor all replicas of a specific app:
```bash
kubectl correlate -l app=payment-processor
```

### 3. Debugging a Rollout
Monitor a rolling deployment in staging while filtering for specific container logs:
```bash
kubectl correlate -n staging -p "^frontend-.*$" --filter "error"
```

---

## Join the Project!

KubeCorrelate is fully open source (Apache 2.0) and written in Go. Check out the repository, run the local incident simulation scripts to see it in action, and tell us what you think!

🔗 **GitHub Repository**: [https://github.com/Dasmat13/kubecorrelate](https://github.com/Dasmat13/kubecorrelate)
