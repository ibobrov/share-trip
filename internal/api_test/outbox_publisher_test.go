package api_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/repository"
	"github.com/ibobrov/share_trip/internal/repository/entity"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

type outboxTestProducer func(context.Context, service.TripPublishedEvent) error

func (p outboxTestProducer) PublishTripPublished(ctx context.Context, event service.TripPublishedEvent) error {
	return p(ctx, event)
}

// Each worker test gets its own schema so it cannot consume another test's outbox.
func newOutboxTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	schema := pgx.Identifier{"outbox_test_" + uuid.New().String()}.Sanitize()
	_, err := testPool.Exec(t.Context(), "CREATE SCHEMA "+schema)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_, err := testPool.Exec(ctx, "DROP SCHEMA "+schema+" CASCADE")
		require.NoError(t, err)
	})
	_, err = testPool.Exec(t.Context(), "CREATE TABLE "+schema+".outbox_event (LIKE public.outbox_event INCLUDING ALL)")
	require.NoError(t, err)
	config := testPool.Config()
	config.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(t.Context(), config)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}

func savePendingEvent(t *testing.T, pool *pgxpool.Pool) service.TripPublishedEvent {
	t.Helper()
	event := service.TripPublishedEvent{
		EventID: uuid.New(), TripID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-acme",
		OccurredAt: time.Now().UTC().Truncate(time.Microsecond),
	}
	payload, err := json.Marshal(entity.PublishTripPayload{
		TripID: event.TripID, ClientID: event.ClientID, CompanyID: event.CompanyID,
	})
	require.NoError(t, err)
	writeTx, err := pool.Begin(t.Context())
	require.NoError(t, err)
	defer func() { _ = writeTx.Rollback(t.Context()) }()
	err = repository.NewOutboxRepository().Create(t.Context(), writeTx, entity.OutboxEvent{
		ID: event.EventID, AggregateID: event.TripID, EventName: domain.TripPublishedEventName,
		Payload: payload, CreatedAt: event.OccurredAt,
	})
	require.NoError(t, err)
	require.NoError(t, writeTx.Commit(t.Context()))
	return event
}

type outboxDeliveryState struct {
	Status    string
	Attempts  int
	LastError *string
	SentAt    *time.Time
}

func readOutboxDelivery(t *testing.T, pool *pgxpool.Pool, id uuid.UUID) outboxDeliveryState {
	t.Helper()
	var state outboxDeliveryState
	err := pool.QueryRow(t.Context(), `SELECT status, attempts, last_error, sent_at FROM outbox_event WHERE id = $1`, id).
		Scan(&state.Status, &state.Attempts, &state.LastError, &state.SentAt)
	require.NoError(t, err)
	return state
}

func TestOutboxPublisher_RetryAfterKafkaFailure(t *testing.T) {
	t.Parallel()
	pool := newOutboxTestPool(t)
	event := savePendingEvent(t, pool)
	kafkaError := errors.New("Kafka unavailable")
	var attempts []service.TripPublishedEvent
	producer := outboxTestProducer(func(_ context.Context, event service.TripPublishedEvent) error {
		attempts = append(attempts, event)
		return kafkaError
	})
	publisher := service.NewOutboxPublisher(pool, repository.NewOutboxRepository(), producer, slog.Default(), service.DefaultOutboxConfig())
	result, err := publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, service.PublishPendingResponse{Failed: 1}, result)
	state := readOutboxDelivery(t, pool, event.EventID)
	require.Equal(t, "pending", state.Status)
	require.Equal(t, 1, state.Attempts)
	require.NotNil(t, state.LastError)
	require.Contains(t, *state.LastError, "Kafka unavailable")
	require.Nil(t, state.SentAt)

	// A new publisher instance resumes the persisted event after restart.
	producer = func(_ context.Context, event service.TripPublishedEvent) error {
		attempts = append(attempts, event)
		return nil
	}
	publisher = service.NewOutboxPublisher(pool, repository.NewOutboxRepository(), producer, slog.Default(), service.DefaultOutboxConfig())
	result, err = publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, service.PublishPendingResponse{Sent: 1}, result)
	require.Equal(t, []service.TripPublishedEvent{event, event}, attempts)
	state = readOutboxDelivery(t, pool, event.EventID)
	require.Equal(t, "sent", state.Status)
	require.Equal(t, 2, state.Attempts)
	require.Nil(t, state.LastError)
	require.NotNil(t, state.SentAt)

	result, err = publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, result)
	require.Len(t, attempts, 2, "sent events must not be selected again")
}

func TestOutboxPublisher_TenFailuresStopAutomaticRetries(t *testing.T) {
	t.Parallel()
	pool := newOutboxTestPool(t)
	event := savePendingEvent(t, pool)
	calls := 0
	producer := outboxTestProducer(func(context.Context, service.TripPublishedEvent) error {
		calls++
		return errors.New("Kafka unavailable")
	})
	publisher := service.NewOutboxPublisher(pool, repository.NewOutboxRepository(), producer, slog.Default(), service.DefaultOutboxConfig())
	for attempt := 1; attempt <= 10; attempt++ {
		result, err := publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
		require.NoError(t, err)
		require.Equal(t, 1, result.Failed)
		require.Equal(t, attempt, readOutboxDelivery(t, pool, event.EventID).Attempts)
	}
	state := readOutboxDelivery(t, pool, event.EventID)
	require.Equal(t, "failed", state.Status)
	require.NotNil(t, state.LastError)
	require.Nil(t, state.SentAt)
	result, err := publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Empty(t, result)
	require.Equal(t, 10, calls)
}

func TestOutboxPublisher_CustomRetryLimit(t *testing.T) {
	t.Parallel()
	pool := newOutboxTestPool(t)
	event := savePendingEvent(t, pool)
	producer := outboxTestProducer(func(context.Context, service.TripPublishedEvent) error {
		return errors.New("Kafka unavailable")
	})
	cfg := service.DefaultOutboxConfig()
	cfg.MaxAttempts = 2
	publisher := service.NewOutboxPublisher(pool, repository.NewOutboxRepository(), producer, slog.Default(), cfg)
	_, err := publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 1})
	require.NoError(t, err)
	require.Equal(t, "pending", readOutboxDelivery(t, pool, event.EventID).Status)
	_, err = publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 1})
	require.NoError(t, err)
	state := readOutboxDelivery(t, pool, event.EventID)
	require.Equal(t, "failed", state.Status)
	require.Equal(t, 2, state.Attempts)
}

func TestOutboxPublisher_ConcurrentWorkersSkipLockedEvents(t *testing.T) {
	t.Parallel()
	pool := newOutboxTestPool(t)
	first := savePendingEvent(t, pool)
	second := savePendingEvent(t, pool)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	started := make(chan service.TripPublishedEvent, 1)
	release := make(chan struct{})
	done := make(chan error, 1)
	blockingProducer := outboxTestProducer(func(ctx context.Context, event service.TripPublishedEvent) error {
		started <- event
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	firstPublisher := service.NewOutboxPublisher(pool, repository.NewOutboxRepository(), blockingProducer, slog.Default(), service.DefaultOutboxConfig())
	go func() {
		_, err := firstPublisher.PublishPending(ctx, service.PublishPendingRequest{Limit: 1})
		done <- err
	}()
	defer func() {
		cancel()
		require.NoError(t, <-done)
	}()
	select {
	case event := <-started:
		require.Equal(t, first.EventID, event.EventID)
	case <-ctx.Done():
		t.Fatal("first publisher did not start")
	}
	var delivered []service.TripPublishedEvent
	secondProducer := outboxTestProducer(func(_ context.Context, event service.TripPublishedEvent) error {
		delivered = append(delivered, event)
		return nil
	})
	secondPublisher := service.NewOutboxPublisher(pool, repository.NewOutboxRepository(), secondProducer, slog.Default(), service.DefaultOutboxConfig())
	result, err := secondPublisher.PublishPending(ctx, service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, service.PublishPendingResponse{Sent: 1}, result)
	require.Equal(t, []service.TripPublishedEvent{second}, delivered)
	close(release)
	// Wait for COMMIT before checking the first event, preserving the deferred join.
	require.Eventually(t, func() bool {
		return readOutboxDelivery(t, pool, first.EventID).Status == "sent"
	}, time.Second, 10*time.Millisecond)
	require.Equal(t, 1, readOutboxDelivery(t, pool, first.EventID).Attempts)
	require.Equal(t, 1, readOutboxDelivery(t, pool, second.EventID).Attempts)
}

type outboxWithMarkSentError struct {
	*repository.OutboxRepository
	err error
}

func (r outboxWithMarkSentError) MarkSent(context.Context, pgx.Tx, uuid.UUID, time.Time) error {
	return r.err
}

func TestOutboxPublisher_KafkaAckBeforeDatabaseFailure(t *testing.T) {
	t.Parallel()
	pool := newOutboxTestPool(t)
	event := savePendingEvent(t, pool)
	var delivered []service.TripPublishedEvent
	producer := outboxTestProducer(func(_ context.Context, event service.TripPublishedEvent) error {
		delivered = append(delivered, event)
		return nil
	})
	dbError := errors.New("database write failed after Kafka ACK")
	outbox := repository.NewOutboxRepository()
	publisher := service.NewOutboxPublisher(pool, outboxWithMarkSentError{outbox, dbError}, producer, slog.Default(), service.DefaultOutboxConfig())
	_, err := publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
	require.ErrorIs(t, err, dbError)
	require.Equal(t, outboxDeliveryState{Status: "pending"}, readOutboxDelivery(t, pool, event.EventID))

	publisher = service.NewOutboxPublisher(pool, outbox, producer, slog.Default(), service.DefaultOutboxConfig())
	_, err = publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, []service.TripPublishedEvent{event, event}, delivered, "redelivery must preserve event_id for inbox deduplication")
	require.Equal(t, "sent", readOutboxDelivery(t, pool, event.EventID).Status)
}

func TestOutboxPublisher_BadPayloadDoesNotBlockOtherEvents(t *testing.T) {
	t.Parallel()
	pool := newOutboxTestPool(t)
	bad := savePendingEvent(t, pool)
	good := savePendingEvent(t, pool)
	_, err := pool.Exec(t.Context(), `UPDATE outbox_event SET payload = '{}' WHERE id = $1`, bad.EventID)
	require.NoError(t, err)
	var delivered []service.TripPublishedEvent
	producer := outboxTestProducer(func(_ context.Context, event service.TripPublishedEvent) error {
		delivered = append(delivered, event)
		return nil
	})
	publisher := service.NewOutboxPublisher(pool, repository.NewOutboxRepository(), producer, slog.Default(), service.DefaultOutboxConfig())
	result, err := publisher.PublishPending(t.Context(), service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, service.PublishPendingResponse{Sent: 1, Failed: 1}, result)
	require.Equal(t, []service.TripPublishedEvent{good}, delivered)
	require.Equal(t, "pending", readOutboxDelivery(t, pool, bad.EventID).Status)
	require.Equal(t, "sent", readOutboxDelivery(t, pool, good.EventID).Status)
}

func TestPublishTrip_OutboxAndTripRollbackTogether(t *testing.T) {
	t.Parallel()
	clientID := uuid.New()
	trip := createDraftForPublishTest(t, clientID)
	before := readPublishTripState(t, trip.ID)
	useCase := domain.NewTripUseCase(repository.NewTripRepository(testPool), repository.NewTripHistoryRepository(), repository.NewOutboxRepository())
	writeTx, err := testPool.Begin(t.Context())
	require.NoError(t, err)
	defer func() { _ = writeTx.Rollback(t.Context()) }()
	publication, err := useCase.MoveTripDraftToPublish(t.Context(), writeTx, domain.PublishTripRequest{
		TripID: trip.ID, ClientID: clientID,
	})
	require.NoError(t, err)
	_, err = useCase.SentNotificationTripPublish(t.Context(), writeTx, domain.SentNotificationTripPublishRequest{
		TripID: publication.TripID, ClientID: publication.ClientID,
		CompanyID: publication.CompanyID, OccurredAt: publication.PublishedAt,
	})
	require.NoError(t, err)
	require.Equal(t, before, readPublishTripState(t, trip.ID), "uncommitted trip and event must not be visible to another connection")
	require.NoError(t, writeTx.Rollback(t.Context()))
	require.Equal(t, before, readPublishTripState(t, trip.ID), "rollback must remove the trip change, history and event together")
}
