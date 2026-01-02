.PHONY: dev tail-log tail-network-log clean help

# Store Makefile directory to allow targets to work from any subdirectory
MAKEFILE_DIR := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))

# Default target
help:
	@echo "Available targets:"
	@echo "  dev              - Start all simulators (auto-reload on file changes)"
	@echo "  tail-log         - Show the last 100 lines of the unified dev log"
	@echo "  tail-network-log - Show the last 100 lines of simulator API request/response logs"
	@echo "  clean            - Clean log files and build artifacts"
	@echo "  docker-up        - Start all simulators via Docker Compose"
	@echo "  docker-down      - Stop all Docker containers"
	@echo "  help             - Show this help message"
	@echo ""

# Start all simulators via Procfile
dev:
	@rm -f .shoreman.pid
	@# Kill any processes on simulator ports
	@lsof -ti:9001 2>/dev/null | xargs kill -9 2>/dev/null || true
	@./scripts/shoreman.sh

# Display the last 100 lines of development log with ANSI codes stripped
tail-log:
	@if [ -f $(MAKEFILE_DIR)dev.log ]; then \
		tail -100 $(MAKEFILE_DIR)dev.log | perl -pe 's/\e\[[0-9;]*m(?:\e\[K)?//g'; \
	else \
		echo "dev.log not found. Run 'make dev' first."; \
	fi

# Display the last 100 lines of simulator API request/response logs
tail-network-log:
	@LOG_FOUND=false; \
	for log in $(MAKEFILE_DIR)simulators/*/simulator.log; do \
		if [ -f "$$log" ]; then \
			LOG_FOUND=true; \
			echo "=== $${log} ==="; \
			tail -100 "$$log"; \
			echo ""; \
		fi; \
	done; \
	if [ "$$LOG_FOUND" = false ]; then \
		echo "No simulator logs found. Make sure simulators are running."; \
	fi

# Clean log files and build artifacts
clean:
	@echo "Cleaning log files and build artifacts..."
	@rm -f dev.log dev-prev.log
	@rm -f simulators/*/simulator.log
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
