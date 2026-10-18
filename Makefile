.DEFAULT_GOAL := help

EXE :=
ifeq ($(OS),Windows_NT)
EXE := .exe
endif

ifeq ($(OS),Windows_NT)
make-dir = powershell.exe -NoProfile -Command "New-Item -ItemType Directory -Force -Path '$(1)' | Out-Null"
else
make-dir = mkdir -p "$(1)"
endif

export GOBIN := $(CURDIR)/.tools

LINT_VERSION := v2.11.3
GOOSE_VERSION := v3.26.0

LINT := $(GOBIN)/golangci-lint$(EXE)
GOOSE := $(GOBIN)/goose$(EXE)

COMPOSE := docker compose --env-file .env -f deploy/docker-compose.yml

REPORTS := .reports

OAPI_CODEGEN_VERSION := v2.8.0
CONTRACT_OPENAPI_URL ?= http://localhost:4002/api/contract.yaml
CONTRACT_CLIENT_DIR := internal/clients/contract

.PHONY: help deps tidy fmt lint test build run up down migrate-up migrate-down migrate-status
.PHONY: migrate-create lint-install coverage coverage-html generate-contract-client

help:
	@echo Available commands:
	@echo   make help               - Show available commands
	@echo   make deps               - Download dependencies and install tools
	@echo   make tidy               - Update go.mod and go.sum
	@echo   make fmt                - Format Go code
	@echo   make lint-install       - Install golangci-lint with Go 1.26.0
	@echo   make lint               - Run linters
	@echo   make test               - Run all tests
	@echo   make coverage           - Run tests and print coverage
	@echo   make coverage-html      - Save HTML coverage report to .reports
	@echo   make build              - Build application into bin
	@echo   make run                - Run application
	@echo   make up                 - Start Docker Compose services
	@echo   make down               - Stop Docker Compose services
	@echo   make migrate-up         - Apply migrations
	@echo   make migrate-down       - Roll back the last migration
	@echo   make migrate-status     - Show migration status
	@echo   make migrate-create name=create_table - Create SQL migration

deps: lint-install
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

lint-install: export GOTOOLCHAIN := go1.26.0
lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
	"$(LINT)" version

coverage:
	$(call make-dir,$(REPORTS))
	go test -count=1 -timeout=5m -coverpkg=./... -covermode=atomic -coverprofile="$(REPORTS)/coverage.out" ./...
	go tool cover -func="$(REPORTS)/coverage.out"

coverage-html: coverage
	go tool cover -html="$(REPORTS)/coverage.out" -o "$(REPORTS)/coverage.html"

generate-contract-client:
	$(call make-dir,$(CONTRACT_CLIENT_DIR)/gen)
	go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION) -generate types,client -response-type-suffix ClientResponse -package contractgen -o "$(CONTRACT_CLIENT_DIR)/gen/client.gen.go" "$(CONTRACT_OPENAPI_URL)"
	go mod tidy
