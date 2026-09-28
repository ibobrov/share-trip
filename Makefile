.DEFAULT_GOAL := help

EXE :=
ifeq ($(OS),Windows_NT)
EXE := .exe
endif

export GOBIN := $(CURDIR)/.tools

LINT_VERSION := v2.11.3
GOOSE_VERSION := v3.26.0

LINT := $(GOBIN)/golangci-lint$(EXE)
GOOSE := $(GOBIN)/goose$(EXE)

COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml

.PHONY: deps tidy fmt lint test build run up down migrate-up migrate-down migrate-status migrate-create

deps:
	go mod download
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
	go install github.com/pressly/goose/v3/cmd/goose@$(GOOSE_VERSION)

tidy:
	go mod tidy

fmt:
	go fmt ./...

lint:
	"$(LINT)" run ./...

test:
	go test ./...

build:
	go build -o bin/sharetrip$(EXE) ./cmd/sharetrip

run:
	go run ./cmd/sharetrip

up:
	$(COMPOSE) up -d --wait

down:
	$(COMPOSE) down

migrate-up:
	"$(GOOSE)" -env .env -dir migrations up

migrate-down:
	"$(GOOSE)" -env .env -dir migrations down

migrate-status:
	"$(GOOSE)" -env .env -dir migrations status

# Default migration name: name. Override with name=create_table.
migrate-create:
	"$(GOOSE)" -dir migrations create "$(if $(strip $(name)),$(strip $(name)),name)" sql