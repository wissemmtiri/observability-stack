package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel/trace"
)

type OrderMessage struct {
	OrderID  int64  `json:"order_id"`
	Item     string `json:"item"`
	Quantity int    `json:"quantity"`
	QueuedAt int64  `json:"queued_at_ms"`
}

type Publisher struct {
	ch    *amqp.Channel
	qName string
}

func newPublisher(amqpURL, queue string) (*Publisher, error) {
	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		return nil, fmt.Errorf("amqp dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("amqp channel: %w", err)
	}
	_, err = ch.QueueDeclare(queue, true, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("queue declare: %w", err)
	}
	return &Publisher{ch: ch, qName: queue}, nil
}

func (p *Publisher) Publish(ctx context.Context, msg OrderMessage) error {
	body, _ := json.Marshal(msg)

	headers := amqp.Table{}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		tp := fmt.Sprintf("00-%s-%s-01", sc.TraceID().String(), sc.SpanID().String())
		headers["traceparent"] = tp
	}

	return p.ch.PublishWithContext(ctx,
		"", p.qName, false, false,
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
			Timestamp:    time.Now(),
			Type:         "OrderEnqueued",
			Headers:      headers,
		},
	)
}
