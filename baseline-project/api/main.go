package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	zlog "github.com/rs/zerolog/log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// Request payload for creating an order
type createOrderRequest struct {
	Item     string
	Quantity int
}

// Custom Prometheus metrics
var (
	requestCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests processed",
		},
		[]string{"method", "path", "status"},
	)
	requestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Request latency in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
)

// Initialize OpenTelemetry Tracer
func initTracer(service string) func(context.Context) error {
	ctx := context.Background()

	// OTEL_EXPORTER_OTLP_ENDPOINT should be set to the OTLP collector endpoint
	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint == "" {
		log.Println("OTEL_EXPORTER_OTLP_ENDPOINT not set; tracing disabled")
		return func(context.Context) error { return nil }
	}

	// Set up OTLP exporter
	exp, err := otlptracehttp.New(
		ctx,
		otlptracehttp.WithEndpoint(endpoint),
		otlptracehttp.WithInsecure(),
	)
	if err != nil {
		log.Fatalf("otel exporter: %v", err)
	}

	// Resource attributes
	res, _ := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(semconv.SchemaURL,
			semconv.ServiceName(service),
			attribute.String("service.namespace", "baseline"),
			attribute.String("deployment.environment", os.Getenv("ENVIRONMENT")),
		),
	)

	// Tracer provider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown
}

func main() {
	// Set up zerolog for structured logging
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	zerolog.SetGlobalLevel(zerolog.InfoLevel)

	// Init Config
	// Initialize Database Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	db, err := initDB(dbURL)
	if err != nil {
		log.Fatalf("db init: %v", err)
	}
	defer db.Close()

	// Initialize RabbitMQ Publisher
	amqpURL := os.Getenv("RABBITMQ_URL")
	if amqpURL == "" {
		log.Fatal("RABBITMQ_URL is required")
	}
	pub, err := newPublisher(amqpURL, "orders")
	if err != nil {
		log.Fatalf("mq init: %v", err)
	}

	shutdown := initTracer("api")
	defer func() { _ = shutdown(context.Background()) }()

	// Create Gin Router
	r := gin.New()
	r.Use(gin.Recovery())
	// Add OpenTelemetry middleware
	r.Use(otelgin.Middleware("api"))
	// Custom request logger middleware
	r.Use(requestLogger())

	// Prometheus metrics middleware
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()

		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		if path == "/metrics" {
			return
		}

		dur := time.Since(start).Seconds()
		code := c.Writer.Status()
		statusText := http.StatusText(code)
		if statusText == "" {
			statusText = fmt.Sprintf("%d", code)
		}

		requestCount.WithLabelValues(c.Request.Method, path, statusText).Inc()
		requestDuration.WithLabelValues(c.Request.Method, path).Observe(dur)
	})

	// Prometheus Metrics endpoint
	prometheus.MustRegister(requestCount, requestDuration)
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// Heathcheck endpoint
	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(
			http.StatusOK,
			gin.H{"status": "ok"},
		)
	})

	// Order Endpoint
	r.POST("/v1/orders", func(c *gin.Context) {
		var req createOrderRequest
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(
				http.StatusBadRequest,
				gin.H{"error": "Item and Quantity are required."},
			)
			return
		}

		tr := otel.Tracer("api")

		// Insert order into DB
		var id int64
		if err := func() error {
			_, span := tr.Start(c.Request.Context(), "db.insert.order",
				trace.WithAttributes(
					attribute.String("db.system", "postgresql"),
					attribute.String("db.operation", "INSERT"),
					attribute.String("db.sql.table", "orders"),
				),
			)
			defer span.End()

			var err error
			id, err = createOrder(c.Request.Context(), db, req.Item, req.Quantity)
			if err != nil {
				span.RecordError(err)
				zlog.Error().Err(err).Str("event", "db_insert_error").Msg("failed to create order")
				return err
			}

			zlog.Info().
				Str("event", "db_insert").
				Str("service", "api").
				Str("item", req.Item).
				Int("quantity", req.Quantity).
				Int64("order_id", id).
				Msg("order_inserted")
			return nil
		}(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "failed to create order",
				"message": "database error",
			})
			return
		}

		// Publish order message to RabbitMQ
		if err := func() error {
			_, span := tr.Start(c.Request.Context(), "messaging.publish.orders",
				trace.WithAttributes(
					attribute.String("messaging.system", "rabbitmq"),
					attribute.String("messaging.destination", "orders"),
					attribute.String("messaging.operation", "publish"),
				),
			)
			defer span.End()

			msg := OrderMessage{
				OrderID:  id,
				Item:     req.Item,
				Quantity: req.Quantity,
				QueuedAt: time.Now().UnixMilli(),
			}
			err := pub.Publish(c.Request.Context(), msg)
			if err != nil {
				span.RecordError(err)
				zlog.Error().Err(err).Str("event", "mq_publish_error").Msg("failed to publish order message")
				return err
			}
			zlog.Info().
				Str("event", "mq_publish").
				Str("service", "api").
				Str("queue", "orders").
				Int64("order_id", id).
				Msg("order_enqueued")
			return nil
		}(); err != nil {
			// **Respond 503**
			c.Header("Retry-After", "10")
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"error":   "failed to enqueue order",
				"message": "temporary queueing failure; please retry",
			})
			return
		}

		// Return success response
		c.JSON(http.StatusAccepted, gin.H{
			"id":        id,
			"item":      req.Item,
			"quantity":  req.Quantity,
			"status":    "received",
			"message":   "enqueued (stub)",
			"step":      1,
			"next_step": "DB insert & message publish",
		})
	})

	// Start Server
	log.Printf("Starting server on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

// Helper middleware for logging request execution time
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		dur := time.Since(start)

		span := trace.SpanFromContext(c.Request.Context())
		sc := span.SpanContext()

		evt := zlog.Info().
			Str("service", "api").
			Str("method", c.Request.Method).
			Str("path", c.FullPath()).
			Str("path", c.FullPath()).
			Int("status", c.Writer.Status()).
			Int64("bytes_out", int64(c.Writer.Size())).
			Int64("latency_ms", dur.Milliseconds())

		if sc.IsValid() {
			evt = evt.
				Str("trace_id", sc.TraceID().String()).
				Str("span_id", sc.SpanID().String())
		}
		evt.Msg("http_request")
	}
}
