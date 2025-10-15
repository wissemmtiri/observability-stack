# Observability Stack Overview 📊

This directory contains the configuration for the complete **observability stack** used to monitor the distributed system. This stack provides the three pillars of observability—**Metrics**, **Logs**, and **Traces**—to give a comprehensive view of the application's performance and health.

---

## 🏗️ Architecture and Components

The stack is defined in the `docker-compose.yml` file and consists of the following services:

| Component | Purpose | Access URL |
| :--- | :--- | :--- |
| **Grafana** | Visualization and Dashboarding | `http://localhost:3000` |
| **Prometheus** | Time-series metrics collection | `http://localhost:9090` |
| **Loki** | Log aggregation and storage | `http://localhost:3100` |
| **Jaeger** | Distributed tracing backend | `http://localhost:16686` |
| **Promtail** | Agent for shipping container logs to Loki | N/A |
| **RabbitMQ Exporter** | Exposes RabbitMQ metrics to Prometheus | N/A |

---

## ⚙️ Service Details

### Metrics Collection (Prometheus)

* **Prometheus** is configured to scrape metrics from the application services (API, Worker) and the **RabbitMQ Exporter**.
* Metrics retention is set to **7 days** to manage disk space in this simulation.
* The **RabbitMQ Exporter** acts as an intermediary, collecting queue and message-rate data from the RabbitMQ management API and exposing it in a format Prometheus can consume.

### Log Aggregation (Loki & Promtail)

* **Loki** is the horizontally scalable, highly available, multi-tenant log aggregation system.
* **Promtail** is deployed alongside Loki. It acts as an agent, discovering and tailing logs from the running application containers on the Docker host (using volume mounts for `/var/run/docker.sock` and `/var/lib/docker/containers`) and forwarding them to Loki.

### Distributed Tracing (Jaeger)

* **Jaeger** is deployed in an **all-in-one** configuration, handling collection, storage, and the UI.
* It is configured to accept traces via the **OTLP** (OpenTelemetry Protocol) endpoint (`4317` and `4318`), which the application services are instrumented to use.

### Visualization (Grafana)

* **Grafana** provides the central interface for analyzing all observability data.
* It is provisioned to automatically connect to **Prometheus** (for metrics) and **Loki** (for logs) as data sources.
* Default administrator credentials are set to `admin`/`admin`.

---

## 🚀 Usage

This stack is intended to be run via the main project's `Makefile`.

To start the observability services only:

```bash
make obs-up