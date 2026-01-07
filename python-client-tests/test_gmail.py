"""
Tests for Gmail API simulator using Python client.
"""
import base64
import pytest
from test_utils import SessionManager, SimulatorClient


@pytest.fixture
def session():
    """Create and cleanup session for each test."""
    manager = SessionManager()
    session_id = manager.create_session()
    yield session_id
    manager.cleanup_session()


@pytest.fixture
def client(session):
    """Create simulator client with session."""
    return SimulatorClient(session)


class TestGmailSendMessage:
    """Test Gmail send message endpoint."""

    def test_send_simple_message(self, client):
        """Test sending a simple text message."""
        # Create RFC 2822 formatted email
        raw_email = "From: sender@example.com\r\n"
        raw_email += "To: recipient@example.com\r\n"
        raw_email += "Subject: Test Email\r\n"
        raw_email += "\r\n"
        raw_email += "This is a test email body."

        # Encode as base64url
        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()

        # Send message
        response = client.post(
            "/gmail/v1/users/me/messages/send",
            json={"raw": encoded}
        )

        assert response.status_code == 200
        data = response.json()
        assert "id" in data
        assert "threadId" in data
        assert data["labelIds"] == ["SENT"]

    def test_send_message_with_html(self, client):
        """Test sending a message with HTML content."""
        raw_email = "From: sender@example.com\r\n"
        raw_email += "To: recipient@example.com\r\n"
        raw_email += "Subject: HTML Email\r\n"
        raw_email += "\r\n"
        raw_email += "<html><body><h1>Hello World</h1></body></html>"

        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()

        response = client.post(
            "/gmail/v1/users/me/messages/send",
            json={"raw": encoded}
        )

        assert response.status_code == 200
        data = response.json()
        assert "id" in data


class TestGmailListMessages:
    """Test Gmail list messages endpoint."""

    def test_list_messages_empty(self, client):
        """Test listing messages when mailbox is empty."""
        response = client.get("/gmail/v1/users/me/messages")

        assert response.status_code == 200
        data = response.json()
        assert "messages" in data or data.get("resultSizeEstimate") == 0

    def test_list_messages_with_pagination(self, client):
        """Test listing messages with maxResults parameter."""
        # Send a few messages first
        for i in range(3):
            raw_email = f"From: sender{i}@example.com\r\n"
            raw_email += "To: recipient@example.com\r\n"
            raw_email += f"Subject: Test {i}\r\n"
            raw_email += "\r\n"
            raw_email += f"Message body {i}"

            encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()
            client.post("/gmail/v1/users/me/messages/send", json={"raw": encoded})

        # List with limit
        response = client.get("/gmail/v1/users/me/messages", params={"maxResults": 2})

        assert response.status_code == 200
        data = response.json()
        assert "resultSizeEstimate" in data

    def test_search_messages_by_from(self, client):
        """Test searching messages by sender."""
        # Send message from specific sender
        raw_email = "From: alice@example.com\r\n"
        raw_email += "To: bob@example.com\r\n"
        raw_email += "Subject: Hello\r\n"
        raw_email += "\r\n"
        raw_email += "Test message"

        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()
        client.post("/gmail/v1/users/me/messages/send", json={"raw": encoded})

        # Search by sender
        response = client.get(
            "/gmail/v1/users/me/messages",
            params={"q": "from:alice@example.com"}
        )

        assert response.status_code == 200
        data = response.json()
        assert data["resultSizeEstimate"] >= 1

    def test_search_messages_by_subject(self, client):
        """Test searching messages by subject."""
        # Send message with specific subject
        raw_email = "From: sender@example.com\r\n"
        raw_email += "To: recipient@example.com\r\n"
        raw_email += "Subject: Important Meeting\r\n"
        raw_email += "\r\n"
        raw_email += "Meeting details"

        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()
        client.post("/gmail/v1/users/me/messages/send", json={"raw": encoded})

        # Search by subject
        response = client.get(
            "/gmail/v1/users/me/messages",
            params={"q": "subject:Important"}
        )

        assert response.status_code == 200
        data = response.json()
        assert data["resultSizeEstimate"] >= 1


class TestGmailGetMessage:
    """Test Gmail get message endpoint."""

    def test_get_message_by_id(self, client):
        """Test retrieving a specific message by ID."""
        # Send a message first
        raw_email = "From: sender@example.com\r\n"
        raw_email += "To: recipient@example.com\r\n"
        raw_email += "Subject: Test Message\r\n"
        raw_email += "\r\n"
        raw_email += "This is the body"

        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()
        send_response = client.post(
            "/gmail/v1/users/me/messages/send",
            json={"raw": encoded}
        )
        message_id = send_response.json()["id"]

        # Get the message
        response = client.get(f"/gmail/v1/users/me/messages/{message_id}")

        assert response.status_code == 200
        data = response.json()
        assert data["id"] == message_id
        assert "payload" in data
        assert "headers" in data["payload"]

        # Verify headers
        headers = {h["name"]: h["value"] for h in data["payload"]["headers"]}
        assert headers["From"] == "sender@example.com"
        assert headers["To"] == "recipient@example.com"
        assert headers["Subject"] == "Test Message"

    def test_get_message_with_parts(self, client):
        """Test that message parts are included in response."""
        raw_email = "From: sender@example.com\r\n"
        raw_email += "To: recipient@example.com\r\n"
        raw_email += "Subject: Test\r\n"
        raw_email += "\r\n"
        raw_email += "Plain text body"

        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()
        send_response = client.post(
            "/gmail/v1/users/me/messages/send",
            json={"raw": encoded}
        )
        message_id = send_response.json()["id"]

        # Get the message
        response = client.get(f"/gmail/v1/users/me/messages/{message_id}")

        assert response.status_code == 200
        data = response.json()
        assert "parts" in data["payload"]
        assert len(data["payload"]["parts"]) >= 1

    def test_get_nonexistent_message(self, client):
        """Test getting a message that doesn't exist."""
        response = client.get("/gmail/v1/users/me/messages/nonexistent-id")

        assert response.status_code == 404


class TestGmailImportMessage:
    """Test Gmail import message endpoint."""

    def test_import_message_with_default_labels(self, client):
        """Test importing a message with default INBOX + UNREAD labels."""
        raw_email = "From: external@example.com\r\n"
        raw_email += "To: me@example.com\r\n"
        raw_email += "Subject: Imported Message\r\n"
        raw_email += "\r\n"
        raw_email += "This message was imported"

        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()

        response = client.post(
            "/gmail/v1/users/me/messages/import",
            json={"raw": encoded}
        )

        assert response.status_code == 200
        data = response.json()
        assert "id" in data
        assert "INBOX" in data["labelIds"]
        assert "UNREAD" in data["labelIds"]

    def test_import_message_with_custom_labels(self, client):
        """Test importing a message with custom labels."""
        raw_email = "From: external@example.com\r\n"
        raw_email += "To: me@example.com\r\n"
        raw_email += "Subject: Custom Labels\r\n"
        raw_email += "\r\n"
        raw_email += "Custom labeled message"

        encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()

        response = client.post(
            "/gmail/v1/users/me/messages/import",
            json={
                "raw": encoded,
                "labelIds": ["INBOX", "IMPORTANT"]
            }
        )

        assert response.status_code == 200
        data = response.json()
        assert "INBOX" in data["labelIds"]
        assert "IMPORTANT" in data["labelIds"]


class TestGmailRateLimit:
    """Test Gmail API rate limiting."""

    def test_rate_limit_not_exceeded(self, client):
        """Test that normal usage doesn't hit rate limits."""
        # Send a few messages
        for i in range(5):
            raw_email = f"From: sender@example.com\r\nTo: recipient@example.com\r\nSubject: Test {i}\r\n\r\nBody"
            encoded = base64.urlsafe_b64encode(raw_email.encode()).decode()
            response = client.post(
                "/gmail/v1/users/me/messages/send",
                json={"raw": encoded}
            )
            assert response.status_code == 200
