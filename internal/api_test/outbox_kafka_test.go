package api_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/events"
	"github.com/ibobrov/share_trip/internal/repository"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestOutboxPublisher_KafkaAndPostgres(t *testing.T) {
	t.Parallel()
	brokerList := os.Getenv("KAFKA_TEST_BROKERS")
	if brokerList == "" {
		t.Skip("set KAFKA_TEST_BROKERS to test real Kafka")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	brokers := strings.Split(brokerList, ",")
	topic := "outbox.publisher.test." + uuid.NewString()
	connection, err := kafka.DialContext(ctx, "tcp", brokers[0])
	require.NoError(t, err)
	defer func() { require.NoError(t, connection.Close()) }()
	require.NoError(t, connection.SetDeadline(time.Now().Add(25*time.Second)))
	require.NoError(t, connection.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 1, ReplicationFactor: 1}))
	defer func() { require.NoError(t, connection.DeleteTopics(topic)) }()
	_, err = connection.ReadPartitions(topic)
	require.NoError(t, err)
	pool := newOutboxTestPool(t)
	event := savePendingEvent(t, pool)
	outbox := repository.NewOutboxRepository()

	// A refused connection must leave a persisted event for the next worker.
	unavailableProducer := events.NewProducer([]string{"127.0.0.1:1"}, topic)
	publisher := service.NewOutboxPublisher(pool, outbox, unavailableProducer, slog.Default())
	result, err := publisher.PublishPending(ctx, service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, service.PublishPendingResponse{Failed: 1}, result)
	require.NoError(t, unavailableProducer.Close())
	require.Equal(t, "pending", readOutboxDelivery(t, pool, event.EventID).Status)

	producer := events.NewProducer(brokers, topic)
	defer func() { require.NoError(t, producer.Close()) }()
	publisher = service.NewOutboxPublisher(pool, outbox, producer, slog.Default())
	result, err = publisher.PublishPending(ctx, service.PublishPendingRequest{Limit: 10})
	require.NoError(t, err)
	require.Equal(t, service.PublishPendingResponse{Sent: 1}, result)

	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: topic, Partition: 0, MaxWait: time.Second})
	defer func() { require.NoError(t, reader.Close()) }()
	require.NoError(t, reader.SetOffset(kafka.FirstOffset))
	message, err := reader.ReadMessage(ctx)
	require.NoError(t, err)
	var body events.TripPublished
	require.NoError(t, json.Unmarshal(message.Value, &body))
	require.Equal(t, event.EventID, body.EventID)
	require.Equal(t, "TripPublished", body.EventType)
	require.Equal(t, event.TripID, body.TripID)
	require.Equal(t, event.ClientID, body.ClientID)
	require.Equal(t, event.CompanyID, body.CompanyID)
	require.True(t, event.OccurredAt.Equal(body.OccurredAt))
	require.Equal(t, event.TripID.String(), string(message.Key))
	state := readOutboxDelivery(t, pool, event.EventID)
	require.Equal(t, "sent", state.Status)
	require.Equal(t, 2, state.Attempts)
	require.Nil(t, state.LastError)
	require.NotNil(t, state.SentAt)
}
