package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/observability/logctx"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/propagation"
)

type OutboxStore interface {
	LockPending(context.Context, pgx.Tx, int) ([]entity.OutboxEvent, error)
	MarkSent(context.Context, pgx.Tx, uuid.UUID, time.Time) error
	MarkFailed(context.Context, pgx.Tx, uuid.UUID, string, int) error
	CountPending(context.Context, pgx.Tx) (int, error)
}

type OutboxConfig struct {
	PollInterval time.Duration
	BatchTimeout time.Duration
	BatchSize    int
	MaxAttempts  int
}

func DefaultOutboxConfig() OutboxConfig {
	return OutboxConfig{PollInterval: time.Second, BatchTimeout: 15 * time.Second, BatchSize: 10, MaxAttempts: 10}
}

// OutboxPublisher is an infrastructure worker; it does not change trip business state.
type OutboxPublisher struct {
	pool     TransactionBeginner
	outbox   OutboxStore
	producer EventPublisher
	logger   *slog.Logger
	config   OutboxConfig
	metrics  *metrics.Metrics
}

func NewOutboxPublisher(pool TransactionBeginner, outbox OutboxStore, producer EventPublisher, logger *slog.Logger, cfg OutboxConfig, observer *metrics.Metrics) *OutboxPublisher {
	return &OutboxPublisher{pool: pool, outbox: outbox, producer: producer, logger: logger, config: cfg, metrics: observer}
}

type PublishPendingRequest struct {
	Limit int
}

type PublishPendingResponse struct {
	Sent   int
	Failed int
}

func (p *OutboxPublisher) Run(ctx context.Context) error {
	ticker := time.NewTicker(p.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			batchCtx, cancel := context.WithTimeout(ctx, p.config.BatchTimeout)
			_, err := p.PublishPending(batchCtx, PublishPendingRequest{Limit: p.config.BatchSize})
			cancel()
			if err != nil && ctx.Err() == nil {
				p.logger.Error("publish outbox batch", "component", "OutboxPublisher", "operation", "PublishBatch", "result", "error", "error", err)
			}
		}
	}
}

func (p *OutboxPublisher) PublishPending(ctx context.Context, req PublishPendingRequest) (PublishPendingResponse, error) {
	if req.Limit <= 0 {
		return PublishPendingResponse{}, fmt.Errorf("outbox batch limit must be positive")
	}
	result, err := tx(ctx, p.pool, func(writeTx pgx.Tx) (*PublishPendingResponse, error) {
		pending, err := p.outbox.LockPending(ctx, writeTx, req.Limit)
		if err != nil {
			return nil, fmt.Errorf("lock pending outbox: %w", err)
		}
		response := &PublishPendingResponse{}
		for _, event := range pending {
			if err := p.publish(ctx, event); err != nil {
				if markErr := p.outbox.MarkFailed(ctx, writeTx, event.ID, err.Error(), p.config.MaxAttempts); markErr != nil {
					return nil, fmt.Errorf("mark outbox failure: %w", markErr)
				}
				response.Failed++
				continue
			}
			if err := p.outbox.MarkSent(ctx, writeTx, event.ID, time.Now().UTC()); err != nil {
				return nil, fmt.Errorf("mark outbox sent: %w", err)
			}
			response.Sent++
		}
		return response, nil
	})
	if err != nil {
		return PublishPendingResponse{}, err
	}
	if p.metrics != nil {
		sampleCtx, cancel := context.WithTimeout(ctx, time.Second)
		defer cancel()
		count, countErr := tx(sampleCtx, p.pool, func(readTx pgx.Tx) (*int, error) {
			count, err := p.outbox.CountPending(sampleCtx, readTx)
			return &count, err
		})
		if countErr != nil {
			p.logger.Error("outbox backlog sampling failed", "component", "OutboxPublisher", "operation", "SampleBacklog", "result", "error", "error", countErr)
		} else {
			p.metrics.OutboxPending.Set(float64(*count))
		}
	}
	return *result, nil
}

func (p *OutboxPublisher) publish(ctx context.Context, event entity.OutboxEvent) (err error) {
	var payload entity.PublishTripPayload
	defer func() {
		result := "success"
		if err != nil {
			result = "error"
		}
		if p.metrics != nil {
			p.metrics.OutboxPublishTotal.WithLabelValues(result).Inc()
			if err != nil {
				p.metrics.OutboxPublishFailed.Inc()
			}
		}
		logCtx := propagation.TraceContext{}.Extract(ctx, propagation.MapCarrier{"traceparent": payload.Traceparent, "tracestate": payload.Tracestate})
		logCtx = logctx.WithLogger(logCtx, p.logger)
		level := slog.LevelInfo
		if err != nil {
			level = slog.LevelError
		}
		logctx.Logger(logCtx).Log(logCtx, level, "outbox publishing attempt completed",
			"component", "OutboxPublisher",
			"operation", "OutboxPublish",
			"event_id", event.ID,
			"trip_id", event.AggregateID,
			"client_id", payload.ClientID,
			"request_id", payload.RequestID,
			"correlation_id", payload.CorrelationID,
			"causation_id", payload.CausationID,
			"traceparent", payload.Traceparent,
			"result", result,
			"error", err,
		)
	}()
	if event.EventName != domain.TripPublishedEventName {
		return fmt.Errorf("unsupported outbox event: %s", event.EventName)
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("decode outbox TripPublished: %w", err)
	}
	if payload.TripID != event.AggregateID || payload.TripID == uuid.Nil || payload.ClientID == uuid.Nil || payload.CompanyID == "" {
		return fmt.Errorf("invalid outbox TripPublished identifiers")
	}
	return p.producer.PublishTripPublished(ctx, TripPublishedEvent{
		EventID: event.ID, TripID: payload.TripID, ClientID: payload.ClientID,
		CompanyID: payload.CompanyID, OccurredAt: event.CreatedAt.UTC(),
		RequestID: payload.RequestID, CorrelationID: payload.CorrelationID, CausationID: payload.CausationID,
		Traceparent: payload.Traceparent, Tracestate: payload.Tracestate,
	})
}
