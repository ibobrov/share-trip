package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/segmentio/kafka-go"
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
	EventID    uuid.UUID `json:"event_id"`
	EventType  string    `json:"event_type"`
	TripID     uuid.UUID `json:"trip_id"`
	ClientID   uuid.UUID `json:"client_id"`
	CompanyID  string    `json:"company_id"`
	OccurredAt time.Time `json:"occurred_at"`
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

func (p *Producer) PublishTripPublished(ctx context.Context, event service.TripPublishedEvent) error {
	data, err := json.Marshal(TripPublished{
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
