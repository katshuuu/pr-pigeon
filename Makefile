APP      := pigeon
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X main.version=$(VERSION)

.PHONY: run build test cover lint certs docker-build up down logs

## run: запустить локально с переменными из .env
run:
	set -a && . ./.env && set +a && go run ./cmd/$(APP)

## build: собрать бинарник в ./bin
build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/$(APP)

## test: тесты с race-детектором
test:
	go test -race -count=1 ./...

## cover: покрытие по функциям и общий процент
cover:
	go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

## lint: golangci-lint (https://golangci-lint.run)
lint:
	golangci-lint run ./...

## certs: самоподписанный сертификат для локального TLS
certs:
	./scripts/gen-certs.sh certs

docker-build:
	docker build -f build/package/Dockerfile --build-arg VERSION=$(VERSION) -t pr-pigeon:$(VERSION) .

## up: бот + postgres в docker compose
up: certs
	VERSION=$(VERSION) docker compose up -d --build

down:
	docker compose down

logs:
	docker compose logs -f pigeon

help:
	@grep -E '^## ' Makefile | sed 's/## //'
