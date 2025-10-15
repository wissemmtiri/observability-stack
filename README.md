# Distributed System with Observability

This repository contains a minimal **distributed system simulation** and an accompanying **observability stack** to monitor its behavior under different load conditions.

The goal is to demonstrate how observability tools can be integrated into a distributed architecture to collect metrics, visualize performance, and detect bottlenecks.

---

## 🧩 Project Structure
This repository is organized into the following main directories and files:

```text
.
├── baseline/           # 📦 Source code for the simulated system
├── observability/      # 📊 Configuration for the monitoring stack 
├── Makefile            # 🤖 Automation for setup, load testing, and scaling
└── README.md           # 📜 This global overview file
```
---

## 🏗️ Architecture Overview

**Baseline System:**
- **API Service** — receives requests and pushes jobs to a message queue (RabbitMQ)
- **Worker Service** — consumes messages from the queue and processes tasks
- **RabbitMQ** — message broker connecting API and workers

**Observability Stack:**
- **Prometheus** — collects metrics from all components
- **Grafana** — visualizes system metrics and queue status
- **Loki** — collect and store logs
- **Jaeger** — provides distributed tracing for request flows

## ⚙️ Getting Started

### 1. Prerequisites
Ensure you have the following installed:
- Docker & Docker Compose
- `make`

### 2. Setup and Run
```bash
# Build and start all components
make obs-up && make app-up
```

### 3. Access Points
- API	http://localhost:8080
- RabbitMQ Management	http://localhost:15672
- Grafana	http://localhost:3000
- Loki http://localhost:3100
- Prometheus	http://localhost:9090
- Jaeger	http://localhost:16686

### 4. Make Automation
You can view all available Make commands at any time by running:
```bash
make
# or
make help
```