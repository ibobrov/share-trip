package config

import (
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	HTTPPort               string
	DatabaseDSN            string
	LogFile                string
	LogLevel               slog.Level
	RequestTimeout         time.Duration
	ContractServiceURL     string
	KafkaBrokers           []string
	TripEventsTopic        string
	ContractRequestTimeout time.Duration
	ContractCheckTimeout   time.Duration
	RetryAttempts          int
	RetryDelay             time.Duration
	KafkaPublishTimeout    time.Duration
	KafkaPublishAttempts   int
	OutboxPollInterval     time.Duration
	OutboxBatchTimeout     time.Duration
	OutboxBatchSize        int
	OutboxMaxAttempts      int
	KeycloakIssuer         string
	KeycloakClientID       string
	KeycloakClientSecret   string
	KeycloakTimeout        time.Duration
	AppEnv                 string
	OTLPEndpoint           string
}

// Load validates configuration before any database, HTTP or Kafka connection is opened.
func Load() (Config, error) {
	var cfg Config
	var err error
	port, err := positiveInt("HTTP_PORT", 8080)
	if err != nil {
		return Config{}, err
	}
	if port > 65535 {
		return Config{}, fmt.Errorf("HTTP_PORT must be between 1 and 65535")
	}
	cfg.HTTPPort = strconv.Itoa(port)
	if cfg.DatabaseDSN, err = databaseDSN(6543); err != nil {
		return Config{}, err
	}
	cfg.LogFile = os.Getenv("LOG_FILE")
	if err = cfg.LogLevel.UnmarshalText([]byte(env("LOG_LEVEL", "INFO"))); err != nil {
		return Config{}, fmt.Errorf("LOG_LEVEL must be DEBUG, INFO, WARN or ERROR")
	}
	if cfg.RequestTimeout, err = milliseconds("REQUEST_TIMEOUT_MS", 2000); err != nil {
		return Config{}, err
	}
	if cfg.KafkaBrokers, err = brokers(); err != nil {
		return Config{}, err
	}
	cfg.TripEventsTopic = env("KAFKA_TRIP_TOPIC", "trip.events")
	if strings.TrimSpace(cfg.TripEventsTopic) == "" {
		return Config{}, fmt.Errorf("KAFKA_TRIP_TOPIC must not be blank")
	}
	if cfg.ContractServiceURL, err = httpURL("CONTRACT_SERVICE_URL"); err != nil {
		return Config{}, err
	}
	if cfg.KeycloakIssuer, err = httpURL("KEYCLOAK_ISSUER"); err != nil {
		return Config{}, err
	}
	if cfg.KeycloakClientSecret, err = required("KEYCLOAK_CLIENT_SECRET"); err != nil {
		return Config{}, err
	}
	cfg.KeycloakClientID = env("KEYCLOAK_CLIENT_ID", "sharetrip-api")
	cfg.AppEnv = env("APP_ENV", "local")
	cfg.OTLPEndpoint = env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:14319")
	if cfg.ContractRequestTimeout, err = milliseconds("CONTRACT_REQUEST_TIMEOUT_MS", 500); err != nil {
		return Config{}, err
	}
	if cfg.ContractCheckTimeout, err = milliseconds("CONTRACT_CHECK_TIMEOUT_MS", 1200); err != nil {
		return Config{}, err
	}
	if cfg.RetryDelay, err = milliseconds("RETRY_DELAY_MS", 100); err != nil {
		return Config{}, err
	}
	if cfg.KafkaPublishTimeout, err = milliseconds("KAFKA_PUBLISH_TIMEOUT_MS", 1000); err != nil {
		return Config{}, err
	}
	if cfg.OutboxPollInterval, err = milliseconds("OUTBOX_POLL_INTERVAL_MS", 1000); err != nil {
		return Config{}, err
	}
	if cfg.OutboxBatchTimeout, err = milliseconds("OUTBOX_BATCH_TIMEOUT_MS", 15000); err != nil {
		return Config{}, err
	}
	if cfg.KeycloakTimeout, err = milliseconds("KEYCLOAK_TIMEOUT_MS", 5000); err != nil {
		return Config{}, err
	}
	if cfg.RetryAttempts, err = positiveInt("RETRY_ATTEMPTS", 2); err != nil {
		return Config{}, err
	}
	if cfg.KafkaPublishAttempts, err = positiveInt("KAFKA_PUBLISH_ATTEMPTS", 3); err != nil {
		return Config{}, err
	}
	if cfg.OutboxBatchSize, err = positiveInt("OUTBOX_BATCH_SIZE", 10); err != nil {
		return Config{}, err
	}
	if cfg.OutboxMaxAttempts, err = positiveInt("OUTBOX_MAX_ATTEMPTS", 10); err != nil {
		return Config{}, err
	}
	if cfg.OutboxMaxAttempts > 2_147_483_647 {
		return Config{}, fmt.Errorf("OUTBOX_MAX_ATTEMPTS must fit PostgreSQL INT")
	}
	if cfg.RetryAttempts > 10 {
		return Config{}, fmt.Errorf("RETRY_ATTEMPTS must not exceed 10")
	}
	if cfg.KafkaPublishAttempts > 10 {
		return Config{}, fmt.Errorf("KAFKA_PUBLISH_ATTEMPTS must not exceed 10")
	}
	if cfg.OutboxBatchSize > 100 {
		return Config{}, fmt.Errorf("OUTBOX_BATCH_SIZE must not exceed 100")
	}
	budget := cfg.ContractCheckTimeout
	for attempt := 0; attempt < cfg.RetryAttempts; attempt++ {
		if budget < cfg.ContractRequestTimeout {
			return Config{}, fmt.Errorf("CONTRACT_CHECK_TIMEOUT_MS must cover all attempts and retry delays")
		}
		budget -= cfg.ContractRequestTimeout
		if attempt+1 < cfg.RetryAttempts {
			if budget < cfg.RetryDelay {
				return Config{}, fmt.Errorf("CONTRACT_CHECK_TIMEOUT_MS must cover all attempts and retry delays")
			}
			budget -= cfg.RetryDelay
		}
	}
	if cfg.ContractCheckTimeout >= cfg.RequestTimeout {
		return Config{}, fmt.Errorf("REQUEST_TIMEOUT_MS must exceed CONTRACT_CHECK_TIMEOUT_MS to leave time for PostgreSQL")
	}
	if cfg.KafkaPublishTimeout > cfg.OutboxBatchTimeout/time.Duration(cfg.OutboxBatchSize) {
		return Config{}, fmt.Errorf("OUTBOX_BATCH_TIMEOUT_MS must cover OUTBOX_BATCH_SIZE Kafka publish timeouts")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func required(key string) (string, error) {
	value := os.Getenv(key)
	if strings.TrimSpace(value) == "" {
		return "", fmt.Errorf("%s is required", key)
	}
	return value, nil
}

func positiveInt(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(env(key, strconv.Itoa(fallback)))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func milliseconds(key string, fallback int) (time.Duration, error) {
	value, err := positiveInt(key, fallback)
	if err != nil {
		return 0, err
	}
	if int64(value) > int64((1<<63-1)/time.Millisecond) {
		return 0, fmt.Errorf("%s is too large", key)
	}
	return time.Duration(value) * time.Millisecond, nil
}

func httpURL(key string) (string, error) {
	value, err := required(key)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return "", fmt.Errorf("%s must be an HTTP(S) URL without credentials", key)
	}
	return value, nil
}

func brokers() ([]string, error) {
	value, err := required("KAFKA_BROKERS")
	if err != nil {
		return nil, err
	}
	result := strings.Split(value, ",")
	for i := range result {
		result[i] = strings.TrimSpace(result[i])
		host, port, err := net.SplitHostPort(result[i])
		number, portErr := strconv.Atoi(port)
		if err != nil || portErr != nil || host == "" || number < 1 || number > 65535 {
			return nil, fmt.Errorf("KAFKA_BROKERS must contain host:port addresses separated by commas")
		}
	}
	return result, nil
}

func databaseDSN(defaultPort int) (string, error) {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		// Existing local Compose environments can still provide separate database variables.
		user, err := required("POSTGRES_USER")
		if err != nil {
			return "", fmt.Errorf("DATABASE_DSN or POSTGRES_USER, POSTGRES_PASSWORD and POSTGRES_DB are required")
		}
		password, err := required("POSTGRES_PASSWORD")
		if err != nil {
			return "", err
		}
		database, err := required("POSTGRES_DB")
		if err != nil {
			return "", err
		}
		port, err := positiveInt("POSTGRES_PORT", defaultPort)
		if err != nil {
			return "", err
		}
		if port > 65535 {
			return "", fmt.Errorf("POSTGRES_PORT must be between 1 and 65535")
		}
		endpoint := url.URL{Scheme: "postgres", User: url.UserPassword(user, password), Host: net.JoinHostPort(env("DB_HOST", "localhost"), strconv.Itoa(port)), Path: "/" + database}
		query := url.Values{"sslmode": {env("DB_SSLMODE", "disable")}}
		endpoint.RawQuery = query.Encode()
		dsn = endpoint.String()
	}
	parsed, err := pgxpool.ParseConfig(dsn)
	if err != nil || parsed.ConnConfig.Database == "" {
		// The parser error may contain the DSN and its password.
		return "", fmt.Errorf("DATABASE_DSN must be a valid PostgreSQL connection string with a database name")
	}
	return dsn, nil
}
