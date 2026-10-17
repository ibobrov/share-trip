package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api"
	"github.com/ibobrov/share_trip/internal/app"
	"github.com/ibobrov/share_trip/internal/domain"
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
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using environment variables")
	}

	ctx := context.Background()

	tp, err := tracing.NewProvider(ctx, tracing.Config{
		ServiceName:    "share-trip",
		ServiceVersion: "1.0.0",
		Environment:    config.Env("APP_ENV", "local"),
		Endpoint:       config.Env("OTEL_EXPORTER_OTLP_ENDPOINT", "localhost:14319"),
	})
	if err != nil {
		log.Printf("init tracing failed: %v", err)
		return
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			5*time.Second,
		)
		defer cancel()

		if err := tp.Shutdown(shutdownCtx); err != nil {
			log.Fatalf("tp.Shutdown: %v", err)
		}
	}()

	dbConfig := repository.Config{
		Host:     config.Env("DB_HOST", "localhost"),
		Port:     config.EnvInt("POSTGRES_PORT", 6543),
		User:     config.Env("POSTGRES_USER", "postgres"),
		Password: config.Env("POSTGRES_PASSWORD", "password"),
		DBName:   config.Env("POSTGRES_DB", "sharetrip"),
		SSLMode:  config.Env("DB_SSLMODE", "disable"),
	}

	pool, err := repository.NewPool(ctx, dbConfig.DSN())
	if err != nil {
		log.Fatalf("repository.NewPool: %v", err)
	}
	defer pool.Close()

	registry := prometheus.NewRegistry()
	tripMetrics := metrics.New(registry)

	tripRepo := repository.NewTripRepository(tripMetrics)
	tripHistoryRepo := repository.NewTripHistoryRepository()
	outboxRepo := repository.NewOutboxRepository()
	tripUseCase := domain.NewTripUseCase(tripRepo, tripHistoryRepo, outboxRepo)
	tripService := service.NewTripService(pool, tripUseCase, tripMetrics)
	server := api.NewServer(pool, tripService, registry)

	application := fiber.New(fiber.Config{
		EnablePrintRoutes: true,
	})

	logger, logFile, err := app.NewLogger()
	if err != nil {
		panic(err)
	}
	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {
			panic(err)
		}
	}(logFile)
	application.Use(tracing.NewFiberMiddleware())
	application.Use(middleware.Correlation(logger))
	application.Use(api.NewHTTPMetricsMiddleware(tripMetrics))

	keycloakClientID := config.Env("KEYCLOAK_CLIENT_ID", "sharetrip-api")
	application.Use(middleware.KeycloakRefreshTokenMiddleware(
		middleware.KeycloakConfig{
			Issuer:       config.Env("KEYCLOAK_ISSUER", "http://localhost:8087/realms/sharetrip"),
			ClientID:     keycloakClientID,
			ClientSecret: config.Env("KEYCLOAK_CLIENT_SECRET", "secret"),
		},
	))

	server.Route(application.Group(""), keycloakClientID)
	httpPort := config.Env("HTTP_PORT", "8080")
	if err := application.Listen(":" + httpPort); err != nil {
		log.Fatalf("application.Listen: %v", err)
		return
	}
}
