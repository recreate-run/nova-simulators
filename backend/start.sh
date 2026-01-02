#!/bin/sh
# Auto-detect environment and run with CompileDaemon
# - Railway: Hot-reload with polling (no PTY needed)
# - Local: Hot-reload with polling (consistent behavior)
#
# CompileDaemon provides hot-reload without requiring PTY access,
# making it perfect for Railway and other containerized environments.

# Ensure CompileDaemon is installed
if ! command -v CompileDaemon &> /dev/null; then
  echo "Installing CompileDaemon..."
  go install github.com/githubnemo/CompileDaemon@latest
fi

if [ -n "$RAILWAY_ENVIRONMENT" ]; then
  echo "🚂 Running on Railway with CompileDaemon (hot-reload, no PTY)..."
else
  echo "💻 Running locally with CompileDaemon (hot-reload)..."
fi

# Run CompileDaemon with polling for file watching
# - polling: Use polling instead of fsnotify (works in Docker/Railway)
# - polling-interval: Check for changes every 500ms
# - log-prefix: Disable CompileDaemon timestamps (app logs its own)
# - build: Command to build the binary (no ldflags optimization for dev)
# - command: Command to run after successful build
exec CompileDaemon \
  -polling \
  -polling-interval=500 \
  -log-prefix=false \
  -build="go build -o ./tmp/main ./cmd/server" \
  -command="./tmp/main"
