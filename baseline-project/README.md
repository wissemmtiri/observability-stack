# Baseline Distributed System Simulation 💻

This directory contains the source code and configuration for the minimal **distributed system simulation**. This application serves as the target for the accompanying observability stack, allowing us to generate metrics, logs, and traces under various load conditions.

---

## 🏗️ Architecture and Components

The baseline system follows a simple producer-consumer pattern, connected by a message broker, and uses a database for persistence.

| Component | Technology / Role | Description |
| :--- | :--- | :--- |
| **API Service** (`api/`) | **Producer/Web** | Receives HTTP requests, persists data to Postgres, and publishes messages to RabbitMQ. |
| **Worker Service** (`worker/`) | **Consumer/Processor** | Consumes messages from RabbitMQ to simulate background job processing. |
| **RabbitMQ** | **Message Broker** | Provides reliable message queuing between the API and Worker services. |
| **PostgreSQL** | **Persistence** | The relational database used by both the API and Worker for state management. |

### 1. Request Flow (Order Creation)

1.  A client sends a **POST** request to the **API Service**.
2.  The API Service records the order in the **PostgreSQL** database.
3.  The API Service publishes an "order" message to the **RabbitMQ** queue.
4.  A **Worker Service** consumes the message from the queue and simulates processing the task.

### 2. Observability Instrumentation

Both the `api` and `worker` services are heavily instrumented to export the three pillars of observability using **Go's standard libraries** and **OpenTelemetry**.

#### **Logs (Loki)**
* Both services use **`zerolog`** for structured logging.
* The log format includes contextual fields like `event`, `service`, `trace_id`, and `span_id`.
* Logs are collected by **Promtail** (in the `observability/` stack) and forwarded to **Loki**.

#### **Metrics (Prometheus)**
* The **API Service** uses **`prometheus/client_golang`** to define and register custom HTTP metrics:
    * `http_requests_total`: A counter for total HTTP requests, labeled by method, path, and status code.
    * `http_request_duration_seconds`: A histogram for request latency, labeled by method and path.
* A dedicated **`/metrics`** endpoint is exposed on port **`8080`** for Prometheus to scrape.

#### **Traces (Jaeger)**
* Instrumentation is handled by **OpenTelemetry (OTEL)**.
* The `api` service uses **`otelgin.Middleware`** to automatically create spans for incoming HTTP requests.
* Custom spans are explicitly created for key operations like:
    * `db.insert.order` (Database interaction)
    * `messaging.publish.orders` (RabbitMQ publishing)
* Trace context is injected into the RabbitMQ message, allowing the **Worker Service** to continue the distributed trace, providing an end-to-end view of the order flow in **Jaeger**.
* The `OTEL_EXPORTER_OTLP_ENDPOINT` environment variable points to the **Jaeger Collector** (`jaeger:4318` for the API).

---

## ⚙️ Running the Baseline System

These services are typically managed via the main project's `Makefile`.

1.  **Ensure the Observability Stack is running:** The services require the external `obsnet` network to connect to Jaeger, Loki, and Prometheus.
2.  **Start the Application:**

```bash
# From the main project root
make app-up