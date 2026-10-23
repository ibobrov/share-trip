package api_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/stretchr/testify/require"
)

func queuedEvents(t *testing.T, tripID uuid.UUID) []entity.OutboxEvent {
	t.Helper()
	rows, err := testPool.Query(t.Context(), `
 SELECT id, event_name, aggregate_id, payload, created_at
 FROM outbox_event WHERE aggregate_id = $1
 `, tripID)
	require.NoError(t, err)
	defer rows.Close()
	var events []entity.OutboxEvent
	for rows.Next() {
		var event entity.OutboxEvent
		require.NoError(t, rows.Scan(&event.ID, &event.EventName, &event.AggregateID, &event.Payload, &event.CreatedAt))
		events = append(events, event)
	}
	require.NoError(t, rows.Err())
	return events
}

func requireQueuedEvent(t *testing.T, tripID, clientID uuid.UUID, companyID string) {
	t.Helper()
	events := queuedEvents(t, tripID)
	require.Len(t, events, 1)
	event := events[0]
	var payload entity.PublishTripPayload
	require.NoError(t, json.Unmarshal(event.Payload, &payload))
	require.Equal(t, clientID, payload.ClientID)
	require.Equal(t, companyID, payload.CompanyID)
	require.Equal(t, tripID, payload.TripID)
	require.Equal(t, tripID, event.AggregateID)
	require.NotEqual(t, uuid.Nil, event.ID)
	require.False(t, event.CreatedAt.IsZero())
	var status string
	var attempts int
	err := testPool.QueryRow(t.Context(), `SELECT status, attempts FROM outbox_event WHERE id = $1`, event.ID).Scan(&status, &attempts)
	require.NoError(t, err)
	require.Equal(t, "pending", status)
	require.Zero(t, attempts, "HTTP publication does not call Kafka")
}
