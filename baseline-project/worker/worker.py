import json, os, time, signal, sys
import pika
import psycopg
from time import sleep 
from opentelemetry import trace
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.trace.propagation.tracecontext import TraceContextTextMapPropagator
import logging, sys
from pythonjsonlogger import jsonlogger
from opentelemetry import trace
from prometheus_client import start_http_server, Counter, Histogram
import time as _time

# Environment Variables
RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
DATABASE_URL = os.getenv("DATABASE_URL", "postgres://app:app@localhost:5432/app")
QUEUE_NAME = os.getenv("QUEUE_NAME", "orders")
WORKER_PORT = int(os.getenv("WORKER_METRICS_PORT", "9100"))
OTEL_ENDPOINT = os.getenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://localhost:4317")
SERVICE_NAME  = os.getenv("OTEL_SERVICE_NAME", "worker")
ENVIRONMENT   = os.getenv("ENVIRONMENT", "dev")

# Metrics
jobs_processed = Counter("worker_jobs_processed_total", "Jobs processed", ["result"])
job_duration   = Histogram("worker_job_duration_seconds", "Job processing duration (s)")
e2e = Histogram("order_e2e_duration_seconds", "API enqueue -> worker processed")

# OpenTelemetry setup
provider = TracerProvider(resource=Resource.create({
    "service.name": SERVICE_NAME,
    "service.namespace": "baseline",
    "deployment.environment": ENVIRONMENT,
}))
provider.add_span_processor(BatchSpanProcessor(
    OTLPSpanExporter(endpoint=OTEL_ENDPOINT, insecure=True)
))
trace.set_tracer_provider(provider)
tracer = trace.get_tracer("worker")

# Text map propagator for context propagation
propagator = TraceContextTextMapPropagator()

# Structured logging setup
logger = logging.getLogger("worker")
logger.setLevel(logging.INFO)

handler = logging.StreamHandler(sys.stdout)
fmt = jsonlogger.JsonFormatter("%(asctime)s %(levelname)s %(message)s")
handler.setFormatter(fmt)
logger.handlers = [handler]
logger.propagate = False

def log_with_trace(level, msg, **fields):
    span = trace.get_current_span()
    sc = span.get_span_context()
    if sc and sc.is_valid:
        fields.update(trace_id=sc.trace_id.to_bytes(16, "big").hex(),
                      span_id=sc.span_id.to_bytes(8, "big").hex())
    getattr(logger, level)(msg, extra={"props": fields})

# Graceful shutdown handling
shutdown = False
def handle_sigterm(signum, frame):
    global shutdown
    shutdown = True

signal.signal(signal.SIGTERM, handle_sigterm)
signal.signal(signal.SIGINT, handle_sigterm)

# RabbitMQ connection
def connect():
    while True:
        try:
            params = pika.URLParameters(RABBITMQ_URL)
            conn = pika.BlockingConnection(params)
            ch = conn.channel()
            ch.queue_declare(queue=QUEUE_NAME, durable=True)
            ch.basic_qos(prefetch_count=1)
            logger.info("worker_connected", extra={"props":{"service":"worker","queue":QUEUE_NAME}})
            return conn, ch
        except Exception as e:
            logger.error("worker_connection_error", extra={"props":{"service":"worker","error":str(e)}})
            time.sleep(2)

# Message processing
def on_message(ch, method, properties, body):
    # Extract tracing context from message headers
    carrier = {}
    if properties and getattr(properties, "headers", None):
        tp = properties.headers.get("traceparent")
        if isinstance(tp, bytes):
            tp = tp.decode("utf-8", errors="ignore")
        if tp:
            carrier["traceparent"] = tp

    parent_ctx = propagator.extract(carrier)
    
    # Start a new span for message processing
    with tracer.start_as_current_span(
        "messaging.consume.orders",
        context=parent_ctx,
        attributes={
            "messaging.system": "rabbitmq",
            "messaging.destination": "orders",
            "messaging.operation": "receive",
        },
    ):
        try:
            msg = json.loads(body.decode() or "{}")
            now_ms = int(_time.time() * 1000)
            if "queued_at_ms" in msg:
                e2e.observe((now_ms - int(msg["queued_at_ms"])) / 1000.0)
            order_id = int(msg.get("order_id", 0))
            if not order_id:
                raise ValueError("missing order_id")
            
            # Log job received
            log_with_trace("info", "job_received", order_id=order_id, queue=QUEUE_NAME)

            # Retry logic for database operation with backoff
            attempts, max_attempts = 0, 3
            while True:
                start_t = _time.time()
                try:
                    with tracer.start_as_current_span("process_order"):
                        mark_processed(order_id)
                        log_with_trace("info", "job_processed", order_id=order_id, result="success")
                    dur = _time.time() - start_t
                    job_duration.observe(dur)
                    jobs_processed.labels(result="success").inc()

                    ch.basic_ack(delivery_tag=method.delivery_tag)
                    return
                except Exception as e:
                    jobs_processed.labels(result="fail").inc()
                    attempts += 1
                    if attempts >= max_attempts:
                        log_with_trace("error", "job_failed", order_id=order_id, attempt=attempts, error=str(e))
                        ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)
                        return
                    sleep(0.5 * attempts)

        except Exception as e:
            log_with_trace("error", "job_invalid", error=str(e), body=body.decode(errors="ignore"))
            ch.basic_nack(delivery_tag=method.delivery_tag, requeue=False)

# Database operation
def mark_processed(order_id: int):
    with psycopg.connect(DATABASE_URL, autocommit=True) as conn:
        with conn.cursor() as cur:
            cur.execute(
                """
                UPDATE orders
                   SET status = 'processed',
                       updated_at = NOW()
                 WHERE id = %s
                """,
                (order_id,),
            )
            if cur.rowcount != 1:
                raise RuntimeError(f"order {order_id} not found")

# Main loop
def main():
    # Start metrics server
    start_http_server(WORKER_PORT)

    # Initial connection
    conn, ch = connect()
    ch.basic_consume(queue=QUEUE_NAME, on_message_callback=on_message)
    print("[worker] consuming… CTRL+C to exit")    

    # Event loop
    while not shutdown:
        try:
            conn.process_data_events(time_limit=1)
        except pika.exceptions.AMQPError as e:
            log_with_trace("error", "rabbitmq_connection_error", error=str(e))
            try:
                conn.close()
            except Exception:
                pass
            conn, ch = connect()

    # Cleanup
    try:
        ch.stop_consuming()
        conn.close()
    except Exception:
        pass
    print("[worker] shutdown complete")

if __name__ == "__main__":
    main()
