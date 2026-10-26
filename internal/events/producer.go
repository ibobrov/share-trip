package events

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/segmentio/kafka-go"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/service"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type ProducerConfig struct {
	PublishTimeout time.Duration
	MaxAttempts    int
}

func DefaultProducerConfig() ProducerConfig {
	return ProducerConfig{PublishTimeout: time.Second, MaxAttempts: 3}
}

type messageWriter interface {
	WriteMessages(context.Context, ...kafka.Message) error
	Close() error
}

type Producer struct {
	writer    messageWriter
	transport *kafka.Transport
	timeout   time.Duration
}

type TripPublished struct {
	RequestID     string    `json:"request_id,omitempty"`
	CorrelationID string    `json:"correlation_id,omitempty"`
	CausationID   string    `json:"causation_id,omitempty"`
	Traceparent   string    `json:"traceparent,omitempty"`
	Tracestate    string    `json:"tracestate,omitempty"`
	EventID       uuid.UUID `json:"event_id"`
	EventType     string    `json:"event_type"`
	TripID        uuid.UUID `json:"trip_id"`
	ClientID      uuid.UUID `json:"client_id"`
	CompanyID     string    `json:"company_id"`
	OccurredAt    time.Time `json:"occurred_at"`
}

func NewProducer(brokers []string, topic string, cfg ProducerConfig) *Producer {
	transport := &kafka.Transport{DialTimeout: cfg.PublishTimeout}
	return &Producer{transport: transport, timeout: cfg.PublishTimeout, writer: &kafka.Writer{
		Addr: kafka.TCP(brokers...), Topic: topic,
		Balancer: &kafka.Hash{}, RequiredAcks: kafka.RequireAll,
		BatchSize: 1, MaxAttempts: cfg.MaxAttempts,
		ReadTimeout: cfg.PublishTimeout, WriteTimeout: cfg.PublishTimeout,
		Transport: transport,
	}}
}

func (p *Producer) PublishTripPublished(ctx context.Context, event service.TripPublishedEvent) (err error) {
	ctx = propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{"traceparent": event.Traceparent, "tracestate": event.Tracestate})
	ctx, span := otel.Tracer("sharetrip").Start(ctx, "Kafka publish TripPublished", trace.WithSpanKind(trace.SpanKindProducer))
	defer span.End()
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "Kafka publish failed")
		}
	}()
	span.SetAttributes(attribute.String("event_id", event.EventID.String()), attribute.String("trip_id", event.TripID.String()), attribute.String("correlation_id", event.CorrelationID))
	correlationID := event.CorrelationID
	if correlationID == "" {
		correlationID = event.EventID.String()
	}
	data, err := json.Marshal(TripPublished{
		RequestID: event.RequestID, CorrelationID: correlationID, CausationID: event.CausationID, Traceparent: event.Traceparent, Tracestate: event.Tracestate,
		EventID: event.EventID, EventType: "TripPublished",
		TripID: event.TripID, ClientID: event.ClientID, CompanyID: event.CompanyID,
		OccurredAt: event.OccurredAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("marshal TripPublished: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()
	if err := p.writer.WriteMessages(ctx, kafka.Message{
		Key: []byte(event.TripID.String()), Value: data, Time: event.OccurredAt.UTC(),
		Headers: []kafka.Header{
			{Key: "event_id", Value: []byte(event.EventID.String())}, {Key: "event_type", Value: []byte("TripPublished")},
			{Key: "request_id", Value: []byte(event.RequestID)}, {Key: "correlation_id", Value: []byte(correlationID)},
			{Key: "causation_id", Value: []byte(event.CausationID)}, {Key: "traceparent", Value: []byte(logctx.Traceparent(ctx))},
			{Key: "tracestate", Value: []byte(logctx.Tracestate(ctx))},
		},
	}); err != nil {
		return fmt.Errorf("publish TripPublished: %w", err)
	}
	return nil
}

func (p *Producer) Close() error {
	if p.transport != nil {
		defer p.transport.CloseIdleConnections()
	}
	return p.writer.Close()
}
