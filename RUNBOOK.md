# Runbook: K8s Observability Stack Alerts

This document provides operational procedures for responding to alerts fired by the K8s Observability Stack. Each section corresponds to a specific Prometheus alert and includes the alert context, impact, diagnostic steps, and resolution actions.

---

## Table of Contents

- [HighQueueBacklog](#highqueuebacklog)
- [PodCrashLooping](#podcrashlooping)
- [General Diagnostic Commands](#general-diagnostic-commands)

---

## HighQueueBacklog

**Severity:** Critical
**Fires when:** `sum(tasks_in_queue) > 30` for more than 2 minutes.

### What This Alert Means

The total number of tasks waiting in the processing queue across all task-processor pods has exceeded 30 and remained above that threshold for at least 2 minutes. This indicates that the application is receiving tasks faster than it can process them. If left unaddressed, the queue will continue to grow, leading to increased latency and potential resource exhaustion.

### Potential Causes

1. **Traffic spike** -- A sudden surge of incoming tasks that exceeds the current processing capacity.
2. **HPA not scaling** -- The HorizontalPodAutoscaler may have reached its `maxReplicas` limit (5) or the custom metrics pipeline (prometheus-adapter) may be broken.
3. **Downstream dependency failure** -- If the task-processor depends on external services, those services may be slow or unavailable, causing tasks to accumulate.
4. **Resource starvation** -- Pods may be throttled due to CPU or memory limits, reducing throughput.

### Diagnostic Steps

#### Step 1: Check current queue depth

```bash
kubectl port-forward -n observability svc/task-processor 8080:80
curl -s http://localhost:8080/metrics | grep tasks_in_queue
```

#### Step 2: Check HPA status

```bash
kubectl get hpa task-processor -n observability
```

Look at the `TARGETS` and `REPLICAS` columns:
- If `REPLICAS` is already at `5/5`, the HPA has maxed out.
- If `TARGETS` shows `<unknown>`, the custom metrics pipeline is broken (check the prometheus-adapter pod).

#### Step 3: Check pod resource usage

```bash
kubectl top pods -n observability -l app=task-processor
```

If pods are near their CPU or memory limits, they may need resource adjustments.

#### Step 4: Check task-processor pod logs

```bash
kubectl logs -n observability -l app=task-processor --tail=100
```

Look for error messages, slow processing indicators, or connection failures to downstream services.

### Resolution Actions

#### Option A: Manually scale the deployment (if HPA has maxed out)

```bash
# Temporarily increase max replicas
kubectl patch hpa task-processor -n observability \
  --type='json' \
  -p='[{"op": "replace", "path": "/spec/maxReplicas", "value": 10}]'

# Or directly scale the deployment (bypasses HPA temporarily)
kubectl scale deployment task-processor -n observability --replicas=8
```

**Important:** If you scale manually past the HPA max, the HPA will scale back down to `maxReplicas` once the metric drops below the threshold. Update the HPA max first if you need sustained higher capacity.

#### Option B: Drain the queue

If tasks are expendable or can be retried later:

```bash
kubectl port-forward -n observability svc/task-processor 8080:80
# Dequeue tasks to reduce backlog
for i in $(seq 1 30); do curl -s -X POST http://localhost:8080/dequeue; done
```

#### Option C: Increase resource limits

If pods are resource-constrained:

```bash
kubectl set resources deployment task-processor -n observability \
  --limits=cpu=200m,memory=256Mi \
  --requests=cpu=100m,memory=128Mi
```

### When to Escalate

- If the queue depth continues to grow after scaling to 10+ replicas.
- If the prometheus-adapter pod is not running or the Custom Metrics API is unavailable.
- If pod logs indicate an issue with an external dependency outside the team's control.

---

## PodCrashLooping

**Severity:** Critical
**Fires when:** `rate(kube_pod_container_status_restarts_total{container="task-processor"}[5m]) > 0` for more than 3 minutes.

### What This Alert Means

One or more task-processor pods are restarting repeatedly (crash-looping). Kubernetes will back off restarts exponentially (CrashLoopBackOff), but the application is fundamentally failing to start or stay running.

### Potential Causes

1. **Application panic** -- An unhandled error in the Go application causing a crash on startup or during request processing.
2. **Configuration error** -- Missing or invalid environment variables, ConfigMap mounts, or secrets.
3. **Resource limits too low** -- The pod is being OOMKilled because the memory limit is insufficient.
4. **Failed health checks** -- The liveness probe is failing, causing Kubernetes to restart the pod even though the application may be partially functional.

### Diagnostic Steps

#### Step 1: Identify the crashing pod

```bash
kubectl get pods -n observability -l app=task-processor
```

Look for pods in `CrashLoopBackOff` or with a high restart count.

#### Step 2: Check pod events

```bash
kubectl describe pod <pod-name> -n observability
```

Look at the `Events` section at the bottom. Key indicators:
- `OOMKilled` -- Memory limit too low.
- `Liveness probe failed` -- Health check endpoint is not responding.
- `Back-off restarting failed container` -- Application is crashing on startup.

#### Step 3: Check logs from the crashed container

```bash
# Current container logs
kubectl logs <pod-name> -n observability

# Previous container logs (if the current one has already restarted)
kubectl logs <pod-name> -n observability --previous
```

#### Step 4: Check resource consumption at crash time

```bash
kubectl top pods -n observability -l app=task-processor
```

### Resolution Actions

#### If OOMKilled

Increase the memory limit:

```bash
kubectl set resources deployment task-processor -n observability \
  --limits=memory=256Mi \
  --requests=memory=128Mi
```

#### If liveness probe failure

Check that the `/healthz` endpoint is responsive:

```bash
kubectl port-forward <pod-name> -n observability 8080:8080
curl -v http://localhost:8080/healthz
```

If the endpoint is slow to respond, increase the probe timeout:

```bash
kubectl patch deployment task-processor -n observability --type='json' \
  -p='[{"op": "replace", "path": "/spec/template/spec/containers/0/livenessProbe/timeoutSeconds", "value": 5}]'
```

#### If application panic

1. Check logs for the stack trace (`kubectl logs <pod-name> -n observability --previous`).
2. Fix the application code in `app/main.go`.
3. Rebuild and redeploy:

```bash
docker build -t task-processor:latest ./app
kind load docker-image task-processor:latest --name observability
kubectl rollout restart deployment task-processor -n observability
```

### When to Escalate

- If the crash is caused by a code bug that requires a developer to fix.
- If the pod cannot start at all and there are no useful logs (possible infrastructure-level issue).

---

## General Diagnostic Commands

### View all resources in the observability namespace

```bash
kubectl get all -n observability
```

### Check cluster-wide events

```bash
kubectl get events -n observability --sort-by='.lastTimestamp' | tail -20
```

### View task-processor pod logs (all pods, streaming)

```bash
kubectl logs -n observability -l app=task-processor -f --max-log-requests=5
```

### Check Prometheus targets

```bash
kubectl port-forward -n observability svc/prometheus 9090:9090
# Open http://localhost:9090/targets
```

### Check Alertmanager status

```bash
kubectl port-forward -n observability svc/alertmanager 9093:9093
# Open http://localhost:9093/#/alerts
```

### Check HPA decisions

```bash
kubectl describe hpa task-processor -n observability
```

The `Events` section shows recent scaling decisions and any errors fetching metrics.

### Force a deployment rollout restart

```bash
kubectl rollout restart deployment task-processor -n observability
kubectl rollout status deployment task-processor -n observability
```
