.PHONY: build run test test-cover lint fmt vet clean docker-build docker-up docker-down

APP_NAME := webhook-buffer
BUILD_DIR := ./bin

build:
	go build -o $(BUILD_DIR)/$(APP_NAME) .

run:
	go run .

test:
	go test ./... -v

test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .
	goimports -w .

vet:
	go vet ./...

clean:
	rm -rf $(BUILD_DIR) coverage.out coverage.html

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f webhook-buffer

deps:
	go mod download
	go mod verify

tidy:
	go mod tidy
