# Nova Simulators

Local API simulators for testing workflow automations in sandbox environments.

## Development Commands

<bash_commands>
make dev                 # Start all simulators (auto-reload on file changes)
make tail-log            # View unified process logs (last 100 lines)
make tail-network-log    # View simulator API request/response logs (last 100 lines)
make clean               # Clean log files and build artifacts
make docker-up           # Start all simulators via Docker Compose
make docker-down         # Stop all Docker containers
make help                # Show all available commands
</bash_commands>

- Do NOT stop the dev server. It stays running and auto-reloads via shoreman, logging to `dev.log`.
- Run `make` from the project's top-level directory.
- You MUST check the tail-log after finishing each task.
- API request/response logs: Full HTTP request bodies (form-encoded), response JSON, timing, and status are logged to `simulators/{service}/simulator.log`.

## Architecture

```
nova-simulators/
├── pkg/
│   ├── transport/      # HTTP interceptor (RoundTripper implementation)
│   └── logging/        # Reusable logging middleware
├── simulators/
│   └── slack/          # Slack API simulator
├── examples/
│   └── slack-demo/     # Demo application
└── scripts/            # Shoreman process manager
```

HTTP Interceptor: Transparent request routing via Go's `http.RoundTripper` interface. Install once, all HTTP calls automatically route to local simulators.

Simulators: HTTP servers that mimic real SaaS APIs (Slack, HubSpot, Jira, etc.). Accept form-encoded POST requests, return JSON responses matching real API behavior.

How It Works:
1. Integration code calls `slack.PostMessage()` → Makes HTTP request to `slack.com`
2. HTTP interceptor catches request → Rewrites URL to `localhost:9001`
3. Slack simulator receives request → Processes and returns JSON response
4. Integration code receives response → Works as if it talked to real Slack

Logging:
- Process logs: `dev.log` (unified output from all simulators)
- API logs: `simulators/slack/simulator.log` (full request/response data with timing)

Key Principle: Existing integration blocks work unchanged. No code modifications needed—just install the HTTP interceptor.
