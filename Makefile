.PHONY: help dev dev-d down down-v logs sh build build-image run \
        migrate-up migrate-down migrate-version migrate-create \
        seed seed-demo test test-integration lint tidy fmt

APP        := agrobench
COMPOSE    := docker compose
API        := $(COMPOSE) exec api
DB_URL_DEV := postgres://agrobench:agrobench@localhost:5432/agrobench?sslmode=disable

help: ## Lista os targets
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'

## ---- ambiente de dev (docker) ----
dev: ## Sobe api (hot-reload) + postgres em foreground
	$(COMPOSE) up --build

dev-d: ## Sobe api + postgres em background
	$(COMPOSE) up --build -d

down: ## Para o ambiente
	$(COMPOSE) down

down-v: ## Para o ambiente e apaga o volume do banco
	$(COMPOSE) down -v

logs: ## Segue os logs da api
	$(COMPOSE) logs -f api

sh: ## Shell dentro do container da api
	$(API) bash

## ---- build ----
build: ## Compila o binário em bin/agrobench
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/$(APP) ./cmd

build-image: ## Builda a imagem de produção
	docker build -t $(APP)-api:latest .

run: build ## Roda o binário local (usa config.dev.json e Postgres do compose)
	./bin/$(APP) serve

## ---- migrations (rodam dentro do container da api) ----
migrate-up: ## Aplica todas as migrations pendentes
	$(API) go run ./cmd migrate up

migrate-down: ## Reverte a última migration
	$(API) go run ./cmd migrate down

migrate-version: ## Mostra a versão atual
	$(API) go run ./cmd migrate version

migrate-create: ## Cria par up/down: make migrate-create name=create_users
	@test -n "$(name)" || (echo "uso: make migrate-create name=<descricao>"; exit 1)
	$(API) migrate create -ext sql -dir migrations -seq $(name)

## ---- seeds ----
seed: ## Dados base (regiões, culturas, admin) — fase 3
	$(API) go run ./cmd seed

seed-demo: ## Dados da demo do pitch — fase 7
	$(API) go run ./cmd seed --demo

## ---- qualidade ----
test: ## Testes unitários
	go test ./... -short -count=1

test-integration: ## Testes de integração (testcontainers, precisa de Docker)
	go test ./... -run Integration -count=1 -v

lint: ## golangci-lint
	golangci-lint run ./...

fmt: ## gofmt + goimports
	gofmt -s -w .

tidy: ## go mod tidy
	go mod tidy
