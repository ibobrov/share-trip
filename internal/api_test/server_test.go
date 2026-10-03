package api_test

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/domain"
	"github.com/ibobrov/share_trip/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/ibobrov/share_trip/internal/api"
	"github.com/ibobrov/share_trip/internal/service"
)

const tripTestURL = "http://localhost/api/trip/"

var (
	testCtx       context.Context
	testDB        *sql.DB
	testPool      *pgxpool.Pool
	testApp       *fiber.App
	testContainer *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	testCtx = context.Background()

	var err error

	testContainer, err = postgres.Run(
		testCtx,
		"postgres:16",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("password"),
	)
	if err != nil {
		log.Fatalf("start postgres container: %v", err)
	}

	dsn, err := testContainer.ConnectionString(
		testCtx,
		"sslmode=disable",
	)
	if err != nil {
		log.Fatalf("get connection string: %v", err)
	}

	testDB, err = sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("open sql db: %v", err)
	}

	waitReady(testDB)

	if err = goose.SetDialect("postgres"); err != nil {
		log.Fatalf("set goose dialect: %v", err)
	}

	if err = goose.Up(testDB, "../../migrations"); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	testPool, err = pgxpool.New(testCtx, dsn)
	if err != nil {
		log.Fatalf("create pgx pool: %v", err)
	}

	tripRepo := repository.NewTripRepository()
	tripHistoryRepo := repository.NewTripHistoryRepository()
	outboxRepo := repository.NewOutboxRepository()
	tripUseCase := domain.NewTripUseCase(tripRepo, tripHistoryRepo, outboxRepo)
	tripService := service.NewTripService(testPool, tripUseCase)
	server := api.NewServer(testPool, tripService)

	testApp = fiber.New()
	server.Route(testApp.Group(""))

	code := m.Run()

	if testPool != nil {
		testPool.Close()
	}
	if testDB != nil {
		_ = testDB.Close()
	}
	if testContainer != nil {
		_ = testContainer.Terminate(testCtx)
	}

	os.Exit(code)
}

func waitReady(db *sql.DB) {
	deadline := time.Now().Add(30 * time.Second)

	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(
			context.Background(),
			2*time.Second,
		)
		err := db.PingContext(ctx)
		cancel()

		if err == nil {
			return
		}

		time.Sleep(500 * time.Millisecond)
	}

	log.Fatalf("database is not ready after timeout")
}
