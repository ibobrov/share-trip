package events_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ibobrov/share_trip/internal/events"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/require"
)

func TestProducer_Kafka(t *testing.T) {
	t.Parallel()
	brokerList := os.Getenv("KAFKA_TEST_BROKERS")
	if brokerList == "" {
		t.Skip("set KAFKA_TEST_BROKERS to run against a real Kafka broker")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	brokers := strings.Split(brokerList, ",")
	topic := "trip.events.test." + uuid.NewString()
	connection, err := kafka.DialContext(ctx, "tcp", brokers[0])
	require.NoError(t, err)
	defer func() { require.NoError(t, connection.Close()) }()
	require.NoError(t, connection.SetDeadline(time.Now().Add(10*time.Second)))
	require.NoError(t, connection.CreateTopics(kafka.TopicConfig{Topic: topic, NumPartitions: 3, ReplicationFactor: 1}))
	defer func() { require.NoError(t, connection.DeleteTopics(topic)) }()
	// Metadata must be available before the producer's one-second publish deadline starts.
	_, err = connection.ReadPartitions(topic)
	require.NoError(t, err)

	producer := events.NewProducer(brokers, topic, events.DefaultProducerConfig())
	defer func() { require.NoError(t, producer.Close()) }()
	event := service.TripPublishedEvent{
		EventID: uuid.New(), TripID: uuid.New(), ClientID: uuid.New(), CompanyID: "company-acme",
		OccurredAt: time.Now().UTC().Truncate(time.Millisecond),
	}
	require.NoError(t, producer.PublishTripPublished(ctx, event))
	partition := (&kafka.Hash{}).Balance(kafka.Message{Key: []byte(event.TripID.String())}, 0, 1, 2)
	reader := kafka.NewReader(kafka.ReaderConfig{Brokers: brokers, Topic: topic, Partition: partition})
	defer func() { require.NoError(t, reader.Close()) }()
	message, err := reader.ReadMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, event.TripID.String(), string(message.Key))
	var received events.TripPublished
	require.NoError(t, json.Unmarshal(message.Value, &received))
	require.Equal(t, "TripPublished", received.EventType)
	require.Equal(t, event.EventID, received.EventID)
	require.Equal(t, event.TripID, received.TripID)
	require.Equal(t, event.ClientID, received.ClientID)
	require.Equal(t, event.CompanyID, received.CompanyID)
	require.True(t, event.OccurredAt.Equal(received.OccurredAt))
}
