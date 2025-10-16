# Timing & Periodic Events

> This file captures only the timing settings we have **in place right now**.

## Snapshot (current)

| Layer | What | Value | Where it’s set | How to verify |
|---|---|---:|---|---|
| Prometheus (global) | `scrape_interval` | **15s** | `observability/prometheus/prometheus.yml` (global) | Prom UI → Status → Configuration |
| Prometheus (global) | `evaluation_interval` | **15s** | `observability/prometheus/prometheus.yml` (global) | Prom UI → Status → Configuration |
| Prometheus — API job | `scrape_interval` | **5s** | `prometheus.yml` → job `api` | Prom UI → Status → Targets (`api`) |
| Prometheus — Worker job | `scrape_interval` | **5s** | `prometheus.yml` → job `worker` | Prom UI → Status → Targets (`worker`) |
| Prometheus — RabbitMQ job | `scrape_interval` | **5s** | `prometheus.yml` → job `rabbitmq` | Prom UI → Status → Targets (`rabbitmq`) |
| Rule group `sli_slo_recording` | `interval` | **15s** | `observability/prometheus/rules/slo_recording.yml` | Prom UI → Status → Rules |
| SLI — API latency p95 | window | **1m** | `slo_recording.yml` (`sli:api_latency:p95:1m`) | Query: `sli:api_latency:p95:1m{path="/v1/orders"}` |
| SLI — API latency p99 | window | **5m** | `slo_recording.yml` (`sli:api_latency:p99:5m`) | Query: `sli:api_latency:p99:5m{path="/v1/orders"}` |
| SLI — API availability | window & coalesce | **5m**, numerator **coalesced to 0** | `slo_recording.yml` (`sli:api_availability:error_ratio:5m`) | Query: `sli:api_availability:error_ratio:5m{path="/v1/orders"}` |
| SLI — Queue depth (instant) | gauge | **instantaneous** | `slo_recording.yml` (`sli:rabbitmq_queue_depth:now`) | Query: `sli:rabbitmq_queue_depth:now` |
| SLI — Queue depth (burst) | max over time | **2m @ 5s step** | `slo_recording.yml` (`sli:rabbitmq_queue_depth:max2m`) | Query: `sli:rabbitmq_queue_depth:max2m` |

## Change Log (timing-only)

| Date (UTC) | Change | Reason |
|---|---|---|
| 2025-10-16 | RabbitMQ scrape **15s → 5s** | Reduce missed short queue spikes |
| 2025-10-16 | Rule group interval **30s → 15s** | Align with eval cadence; fresher SLIs |
| 2025-10-16 | Added `sli:rabbitmq_queue_depth:now` and `:max2m` | Capture live depth and recent bursts |
| 2025-10-16 | Availability SLI coalesce-to-zero on no 5xx | Avoid empty series when no errors |
