.PHONY: build run migrate-up migrate-down migrate-version test test-cover test-cover-html test-cover-core test-cover-core-html clean swagger

COVER_CORE_PKGS := ./internal/handler ./internal/middleware ./internal/repository ./internal/routes ./internal/service ./pkg/validator

# Build the server
build:
	go build -o bin/server ./cmd/server
	go build -o bin/migrate ./cmd/migrate

# Run the server
run:
	go run ./cmd/server

# Run migrations up
migrate-up:
	go run ./cmd/migrate -command=up

# Run migrations down
migrate-down:
	go run ./cmd/migrate -command=down

# Get current migration version
migrate-version:
	go run ./cmd/migrate -command=version

# Run migrations up N steps
migrate-up-steps:
	go run ./cmd/migrate -command=up -steps=$(STEPS)

# Run migrations down N steps
migrate-down-steps:
	go run ./cmd/migrate -command=down -steps=$(STEPS)

# Force migration version
migrate-force:
	go run ./cmd/migrate -command=force -force=$(VERSION)

# Run tests
test:
	go test -v ./...

# Run tests with coverage summary
test-cover:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -n 1

# Run tests with coverage summary and generate HTML report
test-cover-html:
	go test ./... -coverprofile=coverage.out
	go tool cover -func=coverage.out | tail -n 1
	go tool cover -html=coverage.out -o coverage.html
	@echo "Generated coverage report: coverage.html"
	@command -v xdg-open >/dev/null 2>&1 && xdg-open coverage.html >/dev/null 2>&1 || true

# Coverage for core business packages only
test-cover-core:
	go test $(COVER_CORE_PKGS) -coverprofile=coverage.core.out
	go tool cover -func=coverage.core.out | tail -n 1

# Core coverage with HTML report
test-cover-core-html:
	go test $(COVER_CORE_PKGS) -coverprofile=coverage.core.out
	go tool cover -func=coverage.core.out | tail -n 1
	go tool cover -html=coverage.core.out -o coverage.core.html
	@echo "Generated core coverage report: coverage.core.html"
	@command -v xdg-open >/dev/null 2>&1 && xdg-open coverage.core.html >/dev/null 2>&1 || true

# Clean build artifacts
clean:
	rm -rf bin/

# Install dependencies
deps:
	go mod download
	go mod tidy

# Generate Swagger docs
swagger:
	rm -rf docs
	mkdir -p docs
	docker run --rm -u $$(id -u):$$(id -g) -e GOCACHE=/tmp/go-build -e GOPATH=/tmp/go -v "$$(pwd)":/app -w /app golang:1.24 sh -c 'cd cmd/server && go run github.com/swaggo/swag/cmd/swag@v1.8.12 init -g main.go -d .,../../internal/handler,../../internal/dto,../../internal/middleware -o ../../docs --parseDependency --parseInternal'

# Create new migration
# Usage: make create-migration NAME=create_xxx_table
create-migration:
	@mkdir -p migrations
	@touch migrations/$$(printf "%06d" $$(($$(ls migrations/*.up.sql 2>/dev/null | wc -l) + 1)))_$(NAME).up.sql
	@touch migrations/$$(printf "%06d" $$(($$(ls migrations/*.down.sql 2>/dev/null | wc -l) + 1)))_$(NAME).down.sql
	@echo "Created new migration: $(NAME)"
