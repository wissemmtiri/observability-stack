# Metrics Inventory

> What each component emits, when those metrics are updated, and how Prometheus collects them.
> Covers what is implemented **right now**.

---

## Topology

- **API (Go, Gin)** → publishes to **RabbitMQ** (`orders` queue)
- **Worker (Python)** → consumes from **RabbitMQ**
- **Prometheus** scrapes: **API**, **Worker**, **RabbitMQ exporter**

---

## API (Go + Gin)

### Metrics emitted (application)
- `http_requests_total{method, path, status}` *(counter)*
  - `status` is **text** (e.g., `"Accepted"`, `"Bad Request"`, `"Internal Server Error"`).
- `http_request_duration_seconds_bucket{method, path, le}` *(histogram)*  
  - Also `_sum` and `_count`. Units **seconds** (e.g., `0.1` = 100 ms).

### When metrics update
- **Synchronous per HTTP request** in middleware:
  - `Inc()` on `http_requests_total` after response is written
  - `Observe()` on `http_request_duration_seconds` with total request time

### Prometheus collection
- **Endpoint:** `api:8080/metrics`
- **Scrape interval:** **5s** (per-job override)
- **Notes:**
  - `path` is the route label used in SLIs (e.g., `/v1/orders`).
  - Text `status` implies 5xx matching via status **names**.

---

## Worker (Python)

### Metrics emitted (application)
- `worker_jobs_processed_total{result}` *(counter)*
  - `result ∈ {"success","fail"}`
- `worker_job_duration_seconds_bucket{le}` *(histogram)* (+ `_sum`, `_count`)
- `order_e2e_duration_seconds_bucket{le}` *(histogram)* (+ `_sum`, `_count`)  
  - API enqueue → worker processed (observed when message includes `queued_at_ms`)

### When metrics update
- **Synchronous per message** in `on_message`:
  - After successful processing: `job_duration.observe()`, `jobs_processed{result="success"}.inc()`
  - On failure attempt: `jobs_processed{result="fail"}.inc()`
  - On messages with `queued_at_ms`: `e2e.observe()`

### Prometheus collection
- **Endpoint:** `worker:9100/metrics` (via `start_http_server(WORKER_PORT)`)
- **Scrape interval:** **5s**

---

## RabbitMQ (Exporter)

### Metrics emitted
- `rabbitmq_queue_messages_ready{queue}` *(gauge)* — current **ready** depth
- Common additional (not yet used in SLIs):  
  `rabbitmq_queue_messages_unacked{queue}`, `rabbitmq_queue_consumers{queue}`

### When metrics update
- **At scrape time** — exporter reads broker state when Prometheus scrapes

### Prometheus collection
- **Endpoint:** `rabbitmq-exporter:9419/metrics`
- **Scrape interval:** **5s**

---

## Prometheus: collection & SLI recordings

### Global cadence (current)
- `scrape_interval: 15s`
- `evaluation_interval: 15s`

### Per-job overrides (current)
- **API:** 5s
- **Worker:** 5s
- **RabbitMQ exporter:** 5s

### Recording rule group
- **Group:** `sli_slo_recording`
- **Interval:** **15s**

### SLI series produced (current)
- **Latency (API, per path)**  
  - `sli:api_latency:p95:1m{path="/v1/orders"}`  
  - `sli:api_latency:p99:5m{path="/v1/orders"}`
- **Availability (API, per path)**  
  - `sli:api_availability:error_ratio:5m{path="/v1/orders"}`  
    *(5xx matched by **text** status names; numerator coalesces to 0 when none)*
- **Queue depth (RabbitMQ, `orders`)**  
  - Instantaneous: `sli:rabbitmq_queue_depth:now{queue="orders"}`  
  - Burst-resistant: `sli:rabbitmq_queue_depth:max2m{queue="orders"}` *(max over last 2m @ 5s step)*

---

## Verification quick reference

**API latency**
- `sli:api_latency:p95:1m{path="/v1/orders"}`
- `sli:api_latency:p99:5m{path="/v1/orders"}`

**API availability**
- `sli:api_availability:error_ratio:5m{path="/v1/orders"}`  
  *(0 when no errors in the 5m window)*

**Queue depth**
- `rabbitmq_queue_messages_ready{queue="orders"}` *(raw exporter)*
- `sli:rabbitmq_queue_depth:now{queue="orders"}`
- `sli:rabbitmq_queue_depth:max2m{queue="orders"}`

**Targets & rules health**
- Prometheus → **Status → Targets** (API/Worker/RabbitMQ show ~5s last-scrape)
- Prometheus → **Status → Rules** (`sli_slo_recording` interval = 15s)

---

## Notes & next steps

- We currently label API `status` with **text**; consider switching to **numeric** (`"200"`, `"500"`, …) to simplify 5xx selection (`status=~"5.."`).
- Alerts are not included here; separate file will reference the SLI series.
