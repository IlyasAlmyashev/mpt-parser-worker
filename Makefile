.PHONY: build run test clean lint deps

# Variables
BINARY_NAME=mpt-parser-worker
MAIN_FILE=cmd/main.go

# Build the application
build:
	go build -o bin/$(BINARY_NAME) $(MAIN_FILE)

# Run the application
run:
	go run $(MAIN_FILE) --once --only=kaspi

# Run once with Kaspi only
run-once-kaspi:
	go run $(MAIN_FILE) --once --only=kaspi

# Run tests
test:
	go test -v ./...

# Run tests with coverage
test-coverage:
	go test -v -cover ./...

# Clean build artifacts
clean:
	go clean
	rm -rf bin/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Run linter
lint:
	go vet ./...
	@if command -v golangci-lint >/dev/null; then \
		golangci-lint run; \
	else \
		echo "golangci-lint is not installed. Please install it first."; \
		exit 1; \
	fi

# Create new .env file from example if it doesn't exist
init-env:
	@if [ ! -f .env ]; then \
		cp .env.example .env; \
	fi

# Start infrastructure
infra-up:
	docker-compose up -d

# Stop infrastructure
infra-down:
	docker-compose down

# Show infrastructure logs
infra-logs:
	docker-compose logs -f

# Start kafka infrastructure only
kafka-up:
	docker-compose up -d zookeeper kafka kafka-ui

# Stop kafka infrastructure
kafka-down:
	docker-compose stop zookeeper kafka kafka-ui

# Show kafka logs
kafka-logs:
	docker-compose logs -f kafka

# Build docker image
docker-build:
	docker build -t parser-worker .

# Run in docker
docker-run:
	docker run -p 8081:8081 parser-worker

# Help target
help:
	@echo "Available targets:"
	@echo "  build          - Build the application"
	@echo "  run           - Run the application"
	@echo "  run-once-kaspi - Run once with Kaspi marketplace only"
	@echo "  test          - Run tests"
	@echo "  test-coverage - Run tests with coverage"
	@echo "  clean         - Clean build artifacts"
	@echo "  deps          - Install dependencies"
	@echo "  lint          - Run linter"
	@echo "  init-env      - Create .env file from example"
	@echo "  infra-up      - Start infrastructure containers"
	@echo "  infra-down    - Stop infrastructure containers"
	@echo "  infra-logs    - Show infrastructure logs"
	@echo "  docker-build  - Build docker image"
	@echo "  docker-run   - Run in docker container"