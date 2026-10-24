package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api"
	"github.com/ibobrov/share_trip/internal/app"
	"github.com/ibobrov/share_trip/internal/clients/contract"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/events"
	"github.com/ibobrov/share_trip/internal/observability/metrics"
	"github.com/ibobrov/share_trip/internal/observability/middleware"
	"github.com/ibobrov/share_trip/internal/observability/tracing"
	"github.com/ibobrov/share_trip/internal/repository"
	"github.com/ibobrov/share_trip/internal/service"
	"github.com/joho/godotenv"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ibobrov/share_trip/internal/config"
)

func main() {
	if err := run(); err != nil {
		log.Print(err)
		os.Exit(1)
	}
}

func run() error {
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("load .env: invalid or unreadable file")
	}
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("configuration: %w", err)
	}
	logger, logFile, err := app.NewLogger(cfg.LogFile, cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	if logFile != nil {
		defer func() { _ = logFile.Close() }()
	}

	ctx := context.Background()

	tp, err := tracing.NewProvider(ctx, tracing.Config{
		ServiceName:    "share-trip",
		ServiceVersion: "1.0.0",
		Environment:    cfg.AppEnv,
		Endpoint:       cfg.OTLPEndpoint,
	})
	if err != nil {
		return fmt.Errorf("init tracing: %w", err)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Printf("tp.Shutdown: %v", err)
		}
	}()

	pool, err := repository.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		return fmt.Errorf("create postgres pool: %w", err)
	}
	defer pool.Close()

	registry := prometheus.NewRegistry()
	tripMetrics := metrics.New(registry)

	tripRepo := repository.NewTripRepository(pool)
	tripHistoryRepo := repository.NewTripHistoryRepository()
	outboxRepo := repository.NewOutboxRepository()
	tripUseCase := domain.NewTripUseCase(tripRepo, tripHistoryRepo, outboxRepo)
	contractClient, err := contract.NewClient(
		cfg.ContractServiceURL,
		&http.Client{Timeout: cfg.ContractRequestTimeout},
		contract.ClientConfig{
			AttemptTimeout: cfg.ContractRequestTimeout, CheckTimeout: cfg.ContractCheckTimeout,
			RetryAttempts: cfg.RetryAttempts, RetryDelay: cfg.RetryDelay,
		},
	)
	if err != nil {
		return fmt.Errorf("create contract client: %w", err)
	}
	producer := events.NewProducer(cfg.KafkaBrokers, cfg.TripEventsTopic, events.ProducerConfig{
		PublishTimeout: cfg.KafkaPublishTimeout, MaxAttempts: cfg.KafkaPublishAttempts,
	})
	defer func() {
		if err := producer.Close(); err != nil {
			log.Printf("close Kafka producer: %v", err)
		}
	}()
	tripService := service.NewTripService(pool, tripUseCase, tripMetrics, contractClient)
	server := api.NewServer(pool, tripService, registry)

	application := fiber.New(fiber.Config{
		EnablePrintRoutes: true,
	})

	application.Use(tracing.NewFiberMiddleware())
	application.Use(middleware.Correlation(logger))
	application.Use(api.NewHTTPMetricsMiddleware(tripMetrics))

	keycloakClientID := cfg.KeycloakClientID
	application.Use("/api/trip", middleware.KeycloakRefreshTokenMiddleware(
		middleware.KeycloakConfig{
			Issuer:       cfg.KeycloakIssuer,
			ClientID:     keycloakClientID,
			ClientSecret: cfg.KeycloakClientSecret,
			HTTPClient:   &http.Client{Timeout: cfg.KeycloakTimeout},
		},
	))

	server.Route(application.Group(""), keycloakClientID, cfg.RequestTimeout)
	shutdown, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	publisher := service.NewOutboxPublisher(pool, outboxRepo, producer, logger, service.OutboxConfig{
		PollInterval: cfg.OutboxPollInterval, BatchTimeout: cfg.OutboxBatchTimeout, BatchSize: cfg.OutboxBatchSize, MaxAttempts: cfg.OutboxMaxAttempts,
	})
	publisherCtx, stopPublisher := context.WithCancel(shutdown)
	publisherDone := make(chan error, 1)
	go func() { publisherDone <- publisher.Run(publisherCtx) }()
	defer func() {
		stopPublisher()
		if err := <-publisherDone; err != nil && !errors.Is(err, context.Canceled) {
			logger.Error("outbox publisher stopped", "error", err)
		}
	}()
	go func() {
		<-shutdown.Done()
		if err := application.ShutdownWithTimeout(5 * time.Second); err != nil {
			log.Printf("HTTP shutdown: %v", err)
		}
	}()
	if err := application.Listen(":" + cfg.HTTPPort); err != nil {
		return fmt.Errorf("HTTP listen: %w", err)
	}
	return nil
}
