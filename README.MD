# Heartbeat Operator

![Version](https://img.shields.io/badge/version-0.1.0-blue.svg) ![Kubernetes](https://img.shields.io/badge/kubernetes-1.19+-green.svg)
[![CI](https://github.com/EladAviczer/readiness-controller/actions/workflows/ci.yaml/badge.svg)](https://github.com/EladAviczer/readiness-controller/actions/workflows/ci.yaml)

> **Effortless Probing for Kubernetes.**

**Heartbeat Operator** is the easiest way to configure and run health checks, reachability tests, and data validation probes in your Kubernetes cluster.

Stop writing custom scripts, sidecars, or cronjobs just to check if "Site A" is up or "Service B" is reachable. Define your probes declaratively and let the operator handle the rest.

## 🚀 Why Use It?

*   **Simple Configuration**: Define your checks in standard YAML. No code required.
*   **Versatile**: Check internal services, external APIs, databases, or run custom scripts.
*   **Unified Visibility**: Get a consistent view of all your probes via Kubernetes CRDs, Prometheus Metrics, and Grafana.
*   **Zero Overhead**: A single operator manages hundreds of probes efficiently.

## ✨ Features

*   **HTTP/HTTPS**: Check status codes, response times.
*   **TCP**: Check port connectivity (Databases, Queues).
*   **Prometheus**: Native metrics (`probe_success`, `probe_duration_seconds`) on port `9090`.
*   **Grafana**: Includes a ready-to-use dashboard.

## 🛠️ Quick Start

### 1. Install
```bash
helm upgrade --install probe-operator ./charts/probe-operator \
  --namespace default \
  --set metrics.enabled=true
```

### 2. Configure Your Probes
Simply edit `values.yaml` to define what you want to check:

```yaml
probes:
  # Check an external website
  - name: "check-google"
    checkType: "http"
    checkTarget: "https://google.com"
    interval: "30s"

  # Check an internal database port
  - name: "check-postgres"
    checkType: "tcp"
    checkTarget: "postgres-service:5432"
    interval: "10s"
    
  # Check a service in another namespace
  - name: "check-payment-api"
    checkType: "http"
    checkTarget: "http://payment.finance.svc.cluster.local/health"
    interval: "15s"
```

### 3. See Results
**Instant Status via CLI:**
```bash
$ kubectl get probes
NAME               HEALTHY   MESSAGE        AGE
check-google       true      200 OK         2m
check-postgres     true      Connected      2m
check-payment-api  false     503 Unavailable 2m
```

**Visualize in Grafana:**
Import the included `dashboards/grafana-dashboard.json` to see Success Rates, Latency, and Status History instantly.

## 🧩 How It Works
1.  You define a **Probe**.
2.  The **Operator** executes the check (HTTP, TCP, etc.) on the defined interval.
3.  The results are immediately available in:
    *   **Kubernetes API** (as `Probe` objects)
    *   **Prometheus** (as metrics)

**Port Isolation:**
*   `:8080`: Internal status UI.
*   `:9090`: Prometheus metrics.