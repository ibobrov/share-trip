package api_test

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/stretchr/testify/require"
)

type recordingEventPublisher struct {
	mu     sync.Mutex
	events map[uuid.UUID][]service.TripPublishedEvent
}

var testPublisher = &recordingEventPublisher{events: make(map[uuid.UUID][]service.TripPublishedEvent)}

func (p *recordingEventPublisher) PublishTripPublished(ctx context.Context, event service.TripPublishedEvent) error {
	var status string
	if err := testPool.QueryRow(ctx, `SELECT status FROM trips WHERE id = $1`, event.TripID).Scan(&status); err != nil {
		return err
	}
	if status != "published" {
		return fmt.Errorf("event was sent before publication commit: %s", status)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.events[event.TripID] = append(p.events[event.TripID], event)
	return nil
}

func publishedEvents(t *testing.T, tripID uuid.UUID) []service.TripPublishedEvent {
	t.Helper()
	testPublisher.mu.Lock()
	defer testPublisher.mu.Unlock()
	return append([]service.TripPublishedEvent(nil), testPublisher.events[tripID]...)
}

func requirePublishedEvent(t *testing.T, tripID, clientID uuid.UUID, companyID string) {
	t.Helper()
	events := publishedEvents(t, tripID)
	require.Len(t, events, 1)
	event := events[0]
	require.Equal(t, clientID, event.ClientID)
	require.Equal(t, companyID, event.CompanyID)
	require.Equal(t, tripID, event.TripID)
	var eventID uuid.UUID
	err := testPool.QueryRow(t.Context(), `SELECT id FROM outbox_event WHERE aggregate_id = $1`, tripID).Scan(&eventID)
	require.NoError(t, err)
	require.Equal(t, eventID, event.EventID)
	require.False(t, event.OccurredAt.IsZero())
}
