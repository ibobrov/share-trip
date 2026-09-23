package main

import (
	"context"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/ibobrov/share_trip/internal/api"
	"github.com/joho/godotenv"

	"github.com/ibobrov/share_trip/internal/config"
	"github.com/ibobrov/share_trip/internal/repository/postgres"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println(".env file not found, using environment variables")
	}

	ctx := context.Background()

	dbConfig := postgres.Config{
		Host:     config.Env("DB_HOST", "localhost"),
		Port:     config.EnvInt("DB_PORT", 6543),
		User:     config.Env("DB_USER", "postgres"),
		Password: config.Env("DB_PASSWORD", "password"),
		DBName:   config.Env("DB_NAME", "sharetrip"),
		SSLMode:  config.Env("DB_SSLMODE", "disable"),
	}

	pool, err := postgres.NewPool(ctx, dbConfig.DSN())
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	server := api.NewServer(pool)
	app := fiber.New()
	server.Route(app)
	httpPort := config.Env("HTTP_PORT", "8080")
	if err := app.Listen(":" + httpPort); err != nil {
		log.Fatal(err)
	}
}
