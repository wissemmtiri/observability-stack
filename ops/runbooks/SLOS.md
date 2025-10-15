# Service Level Objectives

## API (/v1/orders)
- Availability SLO: 99.9% over 30 days
  - SLI: 1 - (5xx_requests / all_requests)
- Latency SLO: p95 ≤ 100 ms, p99 ≤ 200 ms (over /v1/orders)

## Queue (RabbitMQ "orders")
- Backlog SLO: backlog < 100 messages for ≥ 99% of minutes

## Notes
- Time units: latency metrics are in **seconds**; 0.1s = 100ms.
- Scope: only /v1/orders route for now (to be extended later).
