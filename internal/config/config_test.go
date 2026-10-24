package config

import (
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func validEnvironment(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"HTTP_PORT",
		"APP_ENV",
		"LOG_LEVEL",
		"CONTRACT_SERVICE_URL",
		"KAFKA_BROKERS",
		"KAFKA_TRIP_TOPIC",
		"REQUEST_TIMEOUT_MS",
		"CONTRACT_REQUEST_TIMEOUT_MS",
		"CONTRACT_CHECK_TIMEOUT_MS",
		"RETRY_ATTEMPTS",
		"RETRY_DELAY_MS",
		"KAFKA_PUBLISH_TIMEOUT_MS",
		"KAFKA_PUBLISH_ATTEMPTS",
		"OUTBOX_POLL_INTERVAL_MS",
		"OUTBOX_BATCH_TIMEOUT_MS",
		"OUTBOX_BATCH_SIZE",
		"OUTBOX_MAX_ATTEMPTS",
		"KEYCLOAK_ISSUER",
		"KEYCLOAK_CLIENT_ID",
		"KEYCLOAK_TIMEOUT_MS",
		"OTEL_EXPORTER_OTLP_ENDPOINT",
		"DATABASE_DSN",
		"LOG_FILE",
		"POSTGRES_USER",
		"POSTGRES_PASSWORD",
		"POSTGRES_DB",
		"POSTGRES_PORT",
		"DB_HOST",
		"DB_SSLMODE",
		"KEYCLOAK_CLIENT_SECRET",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("DATABASE_DSN", "postgres://test-user:test-password@localhost:5432/testdb?sslmode=disable")
	t.Setenv("CONTRACT_SERVICE_URL", "http://contract:8080")
	t.Setenv("KEYCLOAK_ISSUER", "http://keycloak:8080/realms/sharetrip")
	t.Setenv("KEYCLOAK_CLIENT_SECRET", "test-keycloak-secret")
	t.Setenv("KAFKA_BROKERS", "localhost:9092, kafka:29092")
}

func TestLoad_ValidConfiguration(t *testing.T) {
	validEnvironment(t)
	t.Setenv("LOG_LEVEL", "DEBUG")
	t.Setenv("REQUEST_TIMEOUT_MS", "3000")
	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, 3*time.Second, cfg.RequestTimeout)
	require.Equal(t, slog.LevelDebug, cfg.LogLevel)
	require.Equal(t, "8080", cfg.HTTPPort)
	require.Equal(t, []string{"localhost:9092", "kafka:29092"}, cfg.KafkaBrokers)
	require.Equal(t, "trip.events", cfg.TripEventsTopic)
	require.Equal(t, 2, cfg.RetryAttempts)
	require.Equal(t, 500*time.Millisecond, cfg.ContractRequestTimeout)
}

func TestLoad_InvalidConfiguration(t *testing.T) {
	tests := []struct{ name, key, value, want string }{
		{"missing database", "DATABASE_DSN", "", "DATABASE_DSN"},
		{"invalid database", "DATABASE_DSN", "postgres://user:must-not-appear@localhost:not-a-port/database", "DATABASE_DSN"},
		{"invalid HTTP port", "HTTP_PORT", "text", "HTTP_PORT"},
		{"HTTP port out of range", "HTTP_PORT", "65536", "HTTP_PORT"},
		{"invalid timeout", "REQUEST_TIMEOUT_MS", "abc", "REQUEST_TIMEOUT_MS"},
		{"negative timeout", "REQUEST_TIMEOUT_MS", "-1", "REQUEST_TIMEOUT_MS"},
		{"duration overflow", "REQUEST_TIMEOUT_MS", "9223372036854775807", "REQUEST_TIMEOUT_MS"},
		{"invalid log level", "LOG_LEVEL", "unknown", "LOG_LEVEL"},
		{"missing brokers", "KAFKA_BROKERS", "", "KAFKA_BROKERS"},
		{"empty broker", "KAFKA_BROKERS", "localhost:9092,", "KAFKA_BROKERS"},
		{"invalid broker port", "KAFKA_BROKERS", "localhost:70000", "KAFKA_BROKERS"},
		{"missing contract URL", "CONTRACT_SERVICE_URL", "", "CONTRACT_SERVICE_URL"},
		{"invalid contract URL", "CONTRACT_SERVICE_URL", "contract:8080", "CONTRACT_SERVICE_URL"},
		{"missing keycloak secret", "KEYCLOAK_CLIENT_SECRET", "", "KEYCLOAK_CLIENT_SECRET"},
		{"secret inside URL", "KEYCLOAK_ISSUER", "http://admin:must-not-appear@keycloak", "KEYCLOAK_ISSUER"},
		{"invalid retries", "RETRY_ATTEMPTS", "0", "RETRY_ATTEMPTS"},
		{"too many retries", "RETRY_ATTEMPTS", "11", "RETRY_ATTEMPTS"},
		{"retries exceed budget", "RETRY_ATTEMPTS", "3", "CONTRACT_CHECK_TIMEOUT_MS"},
		{"no database budget", "REQUEST_TIMEOUT_MS", "1200", "REQUEST_TIMEOUT_MS"},
		{"invalid outbox retries", "OUTBOX_MAX_ATTEMPTS", "0", "OUTBOX_MAX_ATTEMPTS"},
		{"batch too short", "OUTBOX_BATCH_TIMEOUT_MS", "9000", "OUTBOX_BATCH_TIMEOUT_MS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validEnvironment(t)
			t.Setenv(tt.key, tt.value)
			_, err := Load()
			require.ErrorContains(t, err, tt.want)
			require.NotContains(t, err.Error(), "must-not-appear")
			require.NotContains(t, err.Error(), "test-password")
			require.NotContains(t, err.Error(), "test-keycloak-secret")
		})
	}
}

func TestLoad_LocalDatabaseVariables(t *testing.T) {
	validEnvironment(t)
	t.Setenv("DATABASE_DSN", "")
	t.Setenv("POSTGRES_USER", "local-user")
	t.Setenv("POSTGRES_PASSWORD", "password@with:special/characters")
	t.Setenv("POSTGRES_DB", "local-database")
	cfg, err := Load()
	require.NoError(t, err)
	require.Contains(t, cfg.DatabaseDSN, "local-database")
	require.NotContains(t, cfg.DatabaseDSN, "password@with:special/characters")
	t.Setenv("POSTGRES_PASSWORD", "")
	_, err = Load()
	require.ErrorContains(t, err, "POSTGRES_PASSWORD")
}
