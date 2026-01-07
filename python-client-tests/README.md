# Python Client Tests for Nova Simulators

This directory contains Python-based integration tests for Nova Simulators, demonstrating that the simulators work with any HTTP client, not just Go.

## Overview

The tests verify that Python applications can interact with Nova Simulators using standard HTTP requests. Each test creates an isolated session, performs API operations, and validates responses.

## Prerequisites

- Python 3.8 or higher
- Nova Simulators server running on `http://localhost:9000`
- [uv](https://docs.astral.sh/uv/) package manager

## Installation

No installation needed! `uv` will automatically manage dependencies when running tests.

## Running Tests

### Start the Simulator Server

Before running tests, ensure the Nova Simulators server is running:

```bash
# From the project root directory
make dev
```

The server should be running on `http://localhost:9000`.

### Run All Tests

```bash
uv run --with requests --with pytest pytest -v
```

### Run Specific Test File

```bash
# Test Gmail simulator only
uv run --with requests --with pytest pytest test_gmail.py -v

# Test Slack simulator only
uv run --with requests --with pytest pytest test_slack.py -v
```

### Run Specific Test Class

```bash
# Test only Gmail send message functionality
uv run --with requests --with pytest pytest test_gmail.py::TestGmailSendMessage -v

# Test only Slack conversations
uv run --with requests --with pytest pytest test_slack.py::TestSlackConversations -v
```

### Run Specific Test Method

```bash
uv run --with requests --with pytest pytest test_gmail.py::TestGmailSendMessage::test_send_simple_message -v
```

## Test Structure

### test_utils.py

Utility classes for testing:

- **SessionManager**: Creates and cleans up isolated test sessions
- **SimulatorClient**: HTTP client wrapper that automatically includes `X-Session-ID` header

### test_gmail.py

Gmail API simulator tests covering:

- **SendMessage**: Sending plain text and HTML emails
- **ListMessages**: Listing messages with pagination
- **SearchMessages**: Searching by sender, subject, and body text
- **GetMessage**: Retrieving message details with headers and parts
- **ImportMessage**: Importing messages with custom labels
- **RateLimit**: Verifying rate limiting behavior

### test_slack.py

Slack API simulator tests covering:

- **PostMessage**: Posting messages with text, blocks, and formatting
- **Conversations**: Listing channels and conversation history
- **Users**: Listing users and getting user info
- **Auth**: Testing authentication endpoints
- **MultipleMessages**: Testing message ordering and uniqueness

## How It Works

### Session Isolation

Each test creates a unique session ID:

```python
@pytest.fixture
def session():
    manager = SessionManager()
    session_id = manager.create_session()
    yield session_id
    manager.cleanup_session()
```

This ensures tests run independently without data conflicts.

### Making Requests

The `SimulatorClient` automatically includes the session ID:

```python
client = SimulatorClient(session_id)
response = client.get("/gmail/v1/users/me/messages")
```

### Example Test

```python
def test_send_simple_message(self, client):
    # Create email
    raw_email = "From: sender@example.com\r\n"
    raw_email += "To: recipient@example.com\r\n"
    raw_email += "Subject: Test\r\n\r\nBody"

    # Encode and send
    encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()
    response = client.post(
        "/gmail/v1/users/me/messages/send",
        json={"raw": encoded}
    )

    # Validate
    assert response.status_code == 200
    data = response.json()
    assert "id" in data
    assert data["labelIds"] == ["SENT"]
```

## Using Nova Simulators in Your Python Application

### Basic Setup

```python
import requests
import uuid

# Generate unique session ID
session_id = f"my-app-{uuid.uuid4()}"

# Create session
requests.post(
    "http://localhost:9000/api/sessions",
    json={"id": session_id}
)

# Make API calls with session header
headers = {"X-Session-ID": session_id}
response = requests.get(
    "http://localhost:9000/gmail/v1/users/me/messages",
    headers=headers
)

print(response.json())
```

### With Popular Libraries

#### Gmail (using google-api-python-client)

```python
from googleapiclient.discovery import build
import httplib2

# Create custom HTTP client that routes to simulator
http = httplib2.Http()
http.add_credentials(...)  # Add credentials if needed

# Override base URL to point to simulator
service = build('gmail', 'v1', http=http,
                discoveryServiceUrl='http://localhost:9000/gmail/...')
```

#### Slack (using slack-sdk)

```python
from slack_sdk import WebClient

# Point client to simulator
client = WebClient(
    token="fake-token",
    base_url="http://localhost:9000/slack/api"
)

# Use normally
response = client.chat_postMessage(
    channel="#general",
    text="Hello from Python!"
)
```

## API Endpoints

### Session Management

- `POST /api/sessions` - Create new session
- `GET /api/sessions` - List all sessions
- `DELETE /api/sessions/{id}` - Delete session

### Gmail API

- `POST /gmail/v1/users/me/messages/send` - Send message
- `POST /gmail/v1/users/me/messages/import` - Import message
- `GET /gmail/v1/users/me/messages` - List messages
- `GET /gmail/v1/users/me/messages/{id}` - Get message
- `GET /gmail/v1/users/me/messages/{id}/attachments/{attachmentId}` - Get attachment

### Slack API

- `POST /slack/api/chat.postMessage` - Post message
- `GET /slack/api/conversations.list` - List channels
- `GET /slack/api/conversations.history` - Get channel history
- `GET /slack/api/users.list` - List users
- `GET /slack/api/users.info` - Get user info
- `GET /slack/api/auth.test` - Test authentication

## Troubleshooting

### Server Not Running

If tests fail with connection errors:

```
requests.exceptions.ConnectionError: ('Connection aborted.', ConnectionRefusedError(61, 'Connection refused'))
```

Ensure the server is running:
```bash
make dev
```

### Session Conflicts

If you see data from previous tests, the session cleanup may have failed. Restart the server to clear all sessions:

```bash
# Stop server (Ctrl+C)
# Restart
make dev
```

### Rate Limiting

Some tests may hit rate limits if run repeatedly. Check the simulator configuration in `config/simulators.yaml` to adjust rate limits for testing.

## Contributing

To add tests for additional simulators:

1. Create `test_{simulator_name}.py`
2. Follow the existing test structure with session fixtures
3. Use `SimulatorClient` for HTTP requests
4. Test key API endpoints and edge cases
5. Update this README with new endpoints

## License

Same as Nova Simulators project.
