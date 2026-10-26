package events

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/segmentio/kafka-go"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type recordingWriter struct {
	messages []kafka.Message
	err      error
	closed   bool
}

func (w *recordingWriter) WriteMessages(_ context.Context, messages ...kafka.Message) error {
	w.messages = append(w.messages, messages...)
	return w.err
}

func (w *recordingWriter) Close() error {
	w.closed = true
	return w.err
}

func TestProducer_PublishTripPublished(t *testing.T) {
	t.Parallel()
	writer := &recordingWriter{}
	producer := &Producer{writer: writer, timeout: time.Second}
	event := service.TripPublishedEvent{
		RequestID: "request-123", CorrelationID: "publication-456", CausationID: "request-123", Traceparent: "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01",
		EventID: uuid.New(), TripID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-acme",
		OccurredAt: time.Date(2026, 10, 22, 15, 0, 0, 0, time.FixedZone("MSK", 3*60*60)),
	}

	err := producer.PublishTripPublished(t.Context(), event)
	require.NoError(t, err)
	require.Len(t, writer.messages, 1)
	message := writer.messages[0]
	headers := propagation.MapCarrier{}
	for _, header := range message.Headers {
		headers.Set(header.Key, string(header.Value))
	}
	require.Equal(t, event.EventID.String(), headers.Get("event_id"))
	require.Equal(t, "TripPublished", headers.Get("event_type"))
	require.Equal(t, event.CorrelationID, headers.Get("correlation_id"))
	require.Equal(t, event.CausationID, headers.Get("causation_id"))
	remote := propagation.TraceContext{}.Extract(t.Context(), headers)
	require.Equal(t, "4bf92f3577b34da6a3ce929d0e0e4736", trace.SpanContextFromContext(remote).TraceID().String())
	require.Equal(t, event.TripID.String(), string(message.Key))
	var body TripPublished
	require.NoError(t, json.Unmarshal(message.Value, &body))
	require.Equal(t, event.EventID, body.EventID)
	require.Equal(t, "TripPublished", body.EventType)
	require.Equal(t, event.TripID, body.TripID)
	require.Equal(t, event.ClientID, body.ClientID)
	require.Equal(t, event.CompanyID, body.CompanyID)
	require.True(t, event.OccurredAt.Equal(body.OccurredAt))
	require.Equal(t, time.UTC, body.OccurredAt.Location())

	require.NoError(t, producer.Close())
	require.True(t, writer.closed)
}

func TestProducer_PublishTripPublished_Error(t *testing.T) {
	t.Parallel()
	writeError := errors.New("broker unavailable")
	writer := &recordingWriter{err: writeError}
	producer := &Producer{writer: writer, timeout: time.Second}
	err := producer.PublishTripPublished(t.Context(), service.TripPublishedEvent{
		EventID: uuid.New(), TripID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-acme", OccurredAt: time.Now().UTC(),
	})
	require.ErrorIs(t, err, writeError)
	require.Len(t, writer.messages, 1)
}
