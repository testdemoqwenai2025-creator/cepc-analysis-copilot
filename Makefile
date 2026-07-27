.PHONY: dev build run test clean docker-up docker-down migrate

# Go binary path
BIN=bin/server

# Build the Go binary
dev:
	go build -o $(BIN) ./cmd/server/
	./$(BIN)

# Build optimized binary
build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN) ./cmd/server/

# Run tests
test:
	go test ./...

# Run with Docker Compose
docker-up:
	docker compose up --build -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f app

# Clean build artifacts
clean:
	rm -rf bin/ cache/ data/

# Install Python deps
python-deps:
	pip install -r python/requirements.txt

# Database reset (dev only)
migrate:
	docker compose exec db psql -U cepc -c "DROP SCHEMA public CASCADE; CREATE SCHEMA public;"
	docker compose restart app
