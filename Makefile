.PHONY: dev tail-backend-log tail-network-log clean test sqlc-generate help

# Store Makefile directory to allow targets to work from any subdirectory
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Default target
help:
	@echo "Available targets:"
	@echo "  dev              - Start all simulators (auto-reload on file changes)"
	@echo "  test             - Run integration tests (use SIMULATOR=<name> for specific simulator)"
	@echo "  tail-backend-log         - Show the last 100 lines of the unified dev log"
	@echo "  tail-network-log - Show the last 100 lines of simulator API request/response logs"
	@echo "  sqlc-generate    - Generate type-safe Go code from SQL queries"
	@echo "  clean            - Clean log files and build artifacts"
	@echo "  docker-up        - Start all simulators via Docker Compose"
	@echo "  docker-down      - Stop all Docker containers"
	@echo "  help             - Show this help message"
	@echo ""

# Start all simulators via Procfile
dev:
	@rm -f .shoreman.pid
	@# Kill any processes on simulator ports
	@lsof -ti:9000 2>/dev/null | xargs kill -9 2>/dev/null || true
	@./scripts/shoreman.sh

# Run integration tests
# Usage: make test                    (run all tests)
#        make test SIMULATOR=slack    (run specific simulator tests)
test:
	@if [ -n "$(SIMULATOR)" ]; then \
		echo "Running tests for $(SIMULATOR) simulator..."; \
		cd backend && go test ./simulators/$(SIMULATOR) -v; \
	else \
		echo "Running all tests..."; \
		cd backend && go test ./... -v; \
	fi

# Display the last 100 lines of development log with ANSI codes stripped
tail-backend-log:
	@if [ -f $(MAKEFILE_DIR)dev.log ]; then \
		tail -100 $(MAKEFILE_DIR)dev.log | perl -pe 's/\e\[[0-9;]*m(?:\e\[K)?//g'; \
	else \
		echo "dev.log not found. Run 'make dev' first."; \
	fi

# Display the last 100 lines of simulator API request/response logs
tail-network-log:
	@if [ -f $(MAKEFILE_DIR)backend/cmd/server/simulator.log ]; then \
		tail -100 $(MAKEFILE_DIR)backend/cmd/server/simulator.log; \
	else \
		echo "simulator.log not found. Run 'make dev' first."; \
	fi

# Generate type-safe Go code from SQL queries using sqlc
sqlc-generate:
	@echo "Generating type-safe Go code from SQL queries..."
	@cd backend && sqlc generate
	@echo "✅ Code generation complete!"

# Clean log files and build artifacts
clean:
	@echo "Cleaning log files and build artifacts..."
	@rm -f dev.log dev-prev.log
	@rm -f backend/cmd/server/simulator.log
	@rm -f .shoreman.pid
	@echo "✅ Cleaned successfully!"

# Docker Compose targets
docker-up:
	@docker-compose up -d
	@echo "✅ Simulators started via Docker Compose"
	@docker-compose ps

docker-down:
	@docker-compose down
	@echo "✅ Simulators stopped"


# Run linters for both Go backend and frontend
backend-lint:
	@echo "Linting backend ..."
	cd backend && golangci-lint run


# Run linters for both Go backend and frontend
lint: backend-lint 
	@echo "✅ All linting completed successfully!"