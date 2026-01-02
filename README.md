# Nova Simulators

Self-contained API simulators for testing workflow automations in sandbox environments.

## Overview

This repository provides local HTTP simulators that mimic real SaaS APIs (Slack, HubSpot, Jira, etc.) for testing workflow automations without touching production systems.

## Architecture

- **Simulators**: HTTP servers mimicking real API behavior (form-encoded requests, JSON responses)
- **HTTP Interceptor**: Transparent request routing via Go's `http.RoundTripper` interface
- **Logging**: Full request/response audit trail with timing and status codes

## Available Simulators

### Slack (`simulators/slack/`)

**Port**: 9001

**Endpoints**:

- `POST /api/auth.test` - Authentication verification
- `POST /api/chat.postMessage` - Send messages to channels
- `POST /api/conversations.list` - Get channel list
- `POST /api/conversations.history` - Get message history

**Logs to**: `network.log` in simulator directory

## Quick Start

### Option 1: Make Dev (Recommended for Development)

```bash
make dev
```

This starts all simulators with unified logging to `dev.log`. Auto-reloads on file changes.

View logs in real-time:

```bash
make tail-backend-log          # Unified process logs
make tail-network-log  # Simulator API request/response logs
```

### Option 2: Direct Run (Manual)

```bash
cd simulators/slack
go run .
```

### Option 3: Docker Compose (Production)

```bash
make docker-up
```

## Usage with HTTP Interceptor

```go
package main

import (
    "net/http"
    "github.com/recreate-run/nova-simulators/pkg/transport"
    "github.com/slack-go/slack"
)

func main() {
    // Install interceptor
    http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
        "slack.com": "localhost:9001",
    })

    // Use existing Slack client - no code changes needed!
    client := slack.New("fake-token")
    client.PostMessage("C001", slack.MsgOptionText("Hello sandbox!", false))
}
```

## Adding New Simulators

1. Create directory: `simulators/{service}/`
2. Implement HTTP server matching real API behavior
3. Add Dockerfile
4. Update `docker-compose.yml`
5. Document endpoints in this README

## Request/Response Logging

All simulators log full request/response data:

- Timestamp
- Method and path
- Request body (form-encoded)
- Response body (JSON, pretty-printed)
- Duration and status code

View logs: `tail -f simulators/slack/network.log`

## Integration with nova-workflow-starter

In `nova-workflow-starter` automation projects:

1. Start simulators: `cd ../nova-simulators && docker-compose up -d`
2. Install interceptor in your automation code
3. Test automations against local simulators
4. Deploy to production (interceptor disabled)

## License

MIT
