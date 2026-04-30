# K8s Observability and Auto-Scaling Stack

A production-ready Kubernetes observability stack featuring a custom Go application with Prometheus metrics, custom-metrics-driven HPA autoscaling, and Alertmanager routing, fully packaged as a Helm Chart with automated GitHub Actions CI/CD workflows.

## Architecture

```text
[ GitHub Actions ] -- Lint/Test/Scan/Build/Push --> [ GHCR Image Registry ]
        |
     Updates
        v
  [ Helm Chart ] ----- Deploys Stack ----+
                                         |
                                         v
                          +-------------------+
                          |   Grafana         |
                          |   (Dashboards)    |
                          +--------+----------+
                                   |
                          +--------v----------+
                 +------->|   Prometheus      |<------+
                 |        |   (Scrape/Store)  |       |
                 |        +--------+----------+       |
                 |                 |                   |
         +-------+-------+  +-----v------+   +-------+--------+
         | Alertmanager  |  | Prometheus |   | task-processor  |
         | (Routing)     |  | Adapter    |   | (Go App)        |
         +---------------+  +-----+------+   | /metrics        |
                                   |         | /enqueue        |
                            +------v------+  | /dequeue        |
                            | K8s Custom  |  +----------------+
                            | Metrics API |
                            +------+------+
                                   |
                            +------v------+
                            |     HPA     |
                            | (Autoscaler)|
                            +-------------+
```

## Project Structure

```
project/
  app/                        # Go application source
    main.go                   # HTTP server with Prometheus metrics
    main_test.go              # Unit tests for HTTP handlers
    Dockerfile                # Multi-stage container build (Go 1.26 / Alpine 3.20)
    go.mod / go.sum           # Go module dependencies
  chart/                      # Helm Chart for the entire stack
    Chart.yaml
    values.yaml               # Centralized configuration (image repo, replicas)
    templates/
      task-processor.yaml     # Core application & HPA
      prometheus.yaml         # Prometheus server & config
      grafana.yaml            # Grafana & dashboards
      adapter.yaml            # Custom Metrics API bridge
      alertmanager.yaml       # Alerting rules & manager
  .github/workflows/          # GitHub Actions CI/CD
    ci.yaml                   # Lint, test, security scan, Docker build & GHCR push
  RUNBOOK.md                  # Operational runbook for alert response
```

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) or [Minikube](https://minikube.sigs.k8s.io/docs/start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [Helm v3](https://helm.sh/docs/intro/install/)

### 1. Create a Local Cluster

```bash
# Using Kind
kind create cluster --name observability

# Or using Minikube
minikube start --profile observability
```

### 2. Build and Load the Application Image (Local Dev)

*Note: For production, the CI/CD pipeline pushes images to GHCR.*

```bash
# Build the container image locally
docker build -t ghcr.io/jasonjacinth/k8s-observability-stack:latest ./app

# Load into Kind
kind load docker-image ghcr.io/jasonjacinth/k8s-observability-stack:latest --name observability
```

### 3. Deploy the Stack using Helm

Instead of applying raw YAML files one by one, deploy the entire observability stack and application with a single Helm command:

```bash
# Install the chart and create the namespace
helm install observability-stack ./chart -n observability --create-namespace
```

### 4. Verify

```bash
# Check all pods are running (wait a minute for all containers to pull and start)
kubectl get pods -n observability

# Port-forward to test the app
kubectl port-forward -n observability svc/task-processor 8080:80

# In another terminal: enqueue tasks
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/enqueue
curl http://localhost:8080/metrics | grep tasks_in_queue
```

### 5. Access UIs

```bash
# Prometheus UI (http://localhost:9090)
kubectl port-forward -n observability svc/prometheus 9090:9090

# Grafana UI (http://localhost:3000) -- login: admin / admin
kubectl port-forward -n observability svc/grafana 3000:3000

# Alertmanager UI (http://localhost:9093)
kubectl port-forward -n observability svc/alertmanager 9093:9093
```

### 6. Verify HPA and Custom Metrics

```bash
# Check the Custom Metrics API is registered
kubectl get apiservice v1beta1.custom.metrics.k8s.io

# Query the custom metric directly
kubectl get --raw /apis/custom.metrics.k8s.io/v1beta1/namespaces/observability/pods/*/tasks_in_queue | jq .

# Check HPA status (should show "Active")
kubectl get hpa task-processor -n observability

# Watch HPA scaling in real-time
kubectl get hpa task-processor -n observability -w
```

## Custom Metrics

| Metric | Type | Description |
|---|---|---|
| `tasks_in_queue` | Gauge | Current queue depth (drives HPA scaling) |
| `tasks_processed_total` | Counter | Cumulative completed tasks |
| `http_request_duration_seconds` | Histogram | Request latency distribution |

## Alerts

| Alert | Condition | Severity |
|---|---|---|
| `HighQueueBacklog` | `sum(tasks_in_queue) > 30` for 2m | Critical |
| `PodCrashLooping` | Pod restart rate > 0 for 3m | Critical |

See [RUNBOOK.md](RUNBOOK.md) for detailed response procedures.

## Implementation Phases

- **Phase 1**: Go application, Dockerfile, and K8s manifests
- **Phase 2**: Prometheus and Grafana deployment with scrape configuration
- **Phase 3**: Prometheus Adapter and HPA for custom-metric-based autoscaling
- **Phase 4**: Alertmanager rules and operational runbooks
- **Phase 5**: Helm Chart packaging and GitHub Actions CI/CD automation
- **Phase 6**: Shift-Left CI (unit tests, linting, Helm validation, Trivy security scanning)

## License

See [LICENSE](LICENSE) for details.
