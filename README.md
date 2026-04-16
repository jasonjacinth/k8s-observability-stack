# K8s Observability and Auto-Scaling Stack

A production-style Kubernetes observability stack featuring a custom Go application with Prometheus metrics, Grafana dashboards, custom-metrics-driven HPA autoscaling, and Alertmanager-based alerting with operational runbooks.

## Architecture

```
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
    Dockerfile                # Multi-stage container build (Go 1.26 / Alpine 3.20)
    go.mod / go.sum           # Go module dependencies
  k8s/
    base/                     # Core application manifests
      namespace.yaml          # observability namespace
      deployment.yaml         # task-processor Deployment
      service.yaml            # task-processor Service
    prometheus/               # Prometheus server
      prometheus-config.yaml  # Scrape config with K8s SD and alerting
      prometheus-deploy.yaml  # Deployment, RBAC, Service
    grafana/                  # Grafana dashboards
      grafana-deploy.yaml     # Deployment, Service, datasource provisioning
      grafana-dashboard.yaml  # Dashboard JSON (ConfigMap)
    adapter/                  # Custom Metrics API bridge
      prometheus-adapter.yaml # Adapter Deployment, RBAC, APIService
      hpa.yaml                # HorizontalPodAutoscaler (v2)
    alerting/                 # Alertmanager and alert rules
      prometheus-rules.yaml   # Alerting and recording rules
      alertmanager-deploy.yaml # Alertmanager Deployment, Config, Service
  RUNBOOK.md                    # Operational runbook for alert response
```

## Quick Start

### Prerequisites

- [Docker](https://docs.docker.com/get-docker/)
- [Kind](https://kind.sigs.k8s.io/docs/user/quick-start/) or [Minikube](https://minikube.sigs.k8s.io/docs/start/)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)

### 1. Create a Local Cluster

```bash
# Using Kind
kind create cluster --name observability

# Or using Minikube
minikube start --profile observability
```

### 2. Build and Load the Application Image

```bash
# Build the container image
docker build -t task-processor:latest ./app

# Load into Kind
kind load docker-image task-processor:latest --name observability

# Or load into Minikube
minikube image load task-processor:latest --profile observability
```

### 3. Deploy the Application

```bash
kubectl apply -f k8s/base/namespace.yaml
kubectl apply -f k8s/base/deployment.yaml
kubectl apply -f k8s/base/service.yaml
```

### 4. Deploy Prometheus, Grafana, and Alerting

```bash
# Prometheus (with alerting rules and Alertmanager routing)
kubectl apply -f k8s/alerting/prometheus-rules.yaml
kubectl apply -f k8s/prometheus/prometheus-config.yaml
kubectl apply -f k8s/prometheus/prometheus-deploy.yaml

# Grafana
kubectl apply -f k8s/grafana/grafana-dashboard.yaml
kubectl apply -f k8s/grafana/grafana-deploy.yaml

# Alertmanager
kubectl apply -f k8s/alerting/alertmanager-deploy.yaml
```

### 5. Deploy Prometheus Adapter and HPA

```bash
kubectl apply -f k8s/adapter/prometheus-adapter.yaml
kubectl apply -f k8s/adapter/hpa.yaml
```

### 6. Verify

```bash
# Check all pods are running
kubectl get pods -n observability

# Port-forward to test the app
kubectl port-forward -n observability svc/task-processor 8080:80

# In another terminal
curl http://localhost:8080/healthz
curl -X POST http://localhost:8080/enqueue
curl http://localhost:8080/metrics | grep tasks_in_queue
```

### 7. Access UIs

```bash
# Prometheus UI (http://localhost:9090)
kubectl port-forward -n observability svc/prometheus 9090:9090

# Grafana UI (http://localhost:3000) -- login: admin / admin
kubectl port-forward -n observability svc/grafana 3000:3000

# Alertmanager UI (http://localhost:9093)
kubectl port-forward -n observability svc/alertmanager 9093:9093
```

### 8. Verify HPA and Custom Metrics

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

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
