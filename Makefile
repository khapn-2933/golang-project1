.PHONY: build run migrate-up migrate-down migrate-version test clean swagger

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
	docker run --rm -u $$(id -u):$$(id -g) -e GOCACHE=/tmp/go-build -e GOPATH=/tmp/go -v "$$(pwd)":/app -w /app golang:1.24 sh -c 'go run github.com/swaggo/swag/cmd/swag@v1.8.12 init -g main.go -d cmd/server,internal/handler,internal/dto,internal/middleware -o docs --parseDependency --parseInternal'

# Create new migration
# Usage: make create-migration NAME=create_xxx_table
create-migration:
	@mkdir -p migrations
	@touch migrations/$$(printf "%06d" $$(($$(ls migrations/*.up.sql 2>/dev/null | wc -l) + 1)))_$(NAME).up.sql
	@touch migrations/$$(printf "%06d" $$(($$(ls migrations/*.down.sql 2>/dev/null | wc -l) + 1)))_$(NAME).down.sql
	@echo "Created new migration: $(NAME)"
