# Readiness Controller

![Version](https://img.shields.io/badge/version-0.0.4-blue.svg) ![Kubernetes](https://img.shields.io/badge/kubernetes-1.14+-green.svg)
[![CI](https://github.com/EladAviczer/readiness-controller/actions/workflows/ci.yaml/badge.svg)](https://github.com/EladAviczer/readiness-controller/actions/workflows/ci.yaml)

**Readiness Controller** is a lightweight Kubernetes controller that implements **Global Dependency Gating**.

It centralizes health checking for shared resources (Databases, External APIs) and controls traffic to your application Pods using native [Kubernetes Readiness Gates](https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-readiness-gate).

## The Problem
When 50 microservices all depend on a single database, standard Kubernetes Probes create dangerous inefficiencies:

1.  **The "Thundering Herd":** If you have 50 replicas checking the database every 5 seconds, you are hammering your distressed database with **600 requests per minute** just for health checks. This often prevents the database from recovering.
2.  **Slow MTTR (Mean Time To Recovery):** When pods fail their local probes, it is hard to tell if the application is broken or if the dependency is down. Debugging becomes a guessing game.
3.  **Wasted Resources:** Pods consume CPU cycles constantly checking external connections instead of focusing on serving user traffic.

## The Solution
**Readiness Controller** acts as a **Singleton Sentinel**.

1.  **Check Once, Gate Many:** The controller checks the dependency **one time**.
2.  **Global Traffic Control:** If the dependency is down, it closes the "Readiness Gate" on all 50 replicas instantly.
3.  **Reduced Load:** Your application pods stop hammering the database. They simply wait.
4.  **Instant Feedback:** The Status Dashboard shows exactly which Gate is closed, making the root cause obvious immediately.

---

## Features
* **Multi-Gate Support:** Manage multiple dependencies for different microservices from a single controller.
* **Native Integration:** Uses standard K8s `readinessGates`. No sidecars, no Service Mesh required.
* **Pluggable Probers:**
    * `http`: Check REST APIs or Webhooks.
    * `tcp`: Check Databases, Queues, or raw sockets.
    * `exec`: Run custom scripts (curl, grep, jq) for complex logic.
* **Status Dashboard:** Built-in web UI to visualize the state of all gates.

---

## Quick Start

### 1. Prerequisites
* Kubernetes 1.14+
* Helm 3+

### 2. Install the Chart
Clone the repository and install using Helm:

```bash
git clone https://github.com/EladAviczer/readiness-controller.git
cd readiness-controller

# Install the chart
helm install rc ./charts/readiness-controller
```

---

## Integration Guide

For the controller to protect your application, you must explicitly inject the **Readiness Gate** into your Pod Template.

### How to modify your Application Deployment

1.  Open your application's `deployment.yaml`.
2.  Add the `readinessGates` block under `spec.template.spec`.
3.  Ensure the `conditionType` matches exactly what you defined in the controller config values gates section . ```{{ .Values.Gates.gateName }} ```

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-critical-app
spec:
  template:
    spec:
      # --- ADD THIS BLOCK ---
      readinessGates:
        - conditionType: "controller.rc/google-check"
      # ----------------------
      
      containers:
        - name: app
          image: my-app:v1
```

> **Note:** If you forget this step, the controller will run successfully, but it will have **no effect** on your pods. Traffic will continue to flow even if the dependency is down.