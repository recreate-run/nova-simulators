# Nova Simulators

Local API simulators for testing workflow automations in sandbox environments.

## Development Commands

<bash_commands>
make dev                 # Start all simulators (auto-reload on file changes)
make test                # Run all integration tests
make test SIMULATOR=slack # Run tests for specific simulator only
make tail-backend-log            # View unified process logs (last 100 lines)
make tail-network-log    # View simulator API request/response logs (last 100 lines)
make clean               # Clean log files and build artifacts
make docker-up           # Start all simulators via Docker Compose
make docker-down         # Stop all Docker containers
make help                # Show all available commands
</bash_commands>

- Do NOT stop the dev server. It stays running and auto-reloads via shoreman, logging to `dev.log`.
- Run `make` from the project's top-level directory.
- You MUST check the tail-backend-log after finishing each task.
- API request/response logs: Full HTTP request bodies (form-encoded), response JSON, timing, and status are logged to `cmd/server/simulator.log`.

## Architecture

```
nova-simulators/
├── cmd/
│   └── server/         # Unified simulator server (port 9000)
├── pkg/
│   ├── transport/      # HTTP interceptor (RoundTripper implementation)
│   └── logging/        # Reusable logging middleware
├── simulators/
│   └── slack/          # Slack API handler with integration tests
├── examples/
│   └── slack-demo/     # Demo application
└── scripts/            # Shoreman process manager
```

HTTP Interceptor: Transparent request routing via Go's `http.RoundTripper` interface. Install once, all HTTP calls automatically route to local simulators.

Simulators: Single HTTP server (port 9000) with path-based routing for multiple SaaS APIs. Each simulator is a handler module (e.g., `/slack`, `/hubspot`). Accept form-encoded POST requests, return JSON responses matching real API behavior.

Sessions: All requests require `X-Session-ID` header for multi-tenant isolation—each session has independent data with no predetermined seed state.

How It Works:

1. Integration code calls `slack.PostMessage()` → Makes HTTP request to `slack.com`
2. HTTP interceptor catches request → Rewrites URL to `localhost:9000/slack`
3. Slack handler receives request → Processes and returns JSON response
4. Integration code receives response → Works as if it talked to real Slack

Testing:

- Run `make test` to execute all integration tests
- Run `make test SIMULATOR=slack` for specific simulator tests
- Tests use `httptest.NewServer()` for isolated testing without manual server startup
- Seed tests (`seed_test.go`) demonstrate SQL-only seeding pattern—create all initial state (channels, users, messages) via database queries without HTTP/API calls

Logging:

- Process logs: `dev.log` (unified output from all simulators)
- API logs: `cmd/server/simulator.log` (full request/response data with timing, tagged by simulator name)

Key Principle: Existing integration blocks work unchanged. No code modifications needed—just install the HTTP interceptor.
