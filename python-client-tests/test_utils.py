"""
Utility functions for testing Nova Simulators with Python clients.
"""
import requests
import uuid


BASE_URL = "http://localhost:9000"


class SessionManager:
    """Manages session creation and cleanup for isolated testing."""

    def __init__(self, base_url=BASE_URL):
        self.base_url = base_url
        self.session_id = None

    def create_session(self):
        """Create a new session and return the session ID."""
        self.session_id = f"python-test-{uuid.uuid4()}"
        response = requests.post(
            f"{self.base_url}/api/sessions",
            json={"id": self.session_id}
        )
        response.raise_for_status()
        return self.session_id

    def cleanup_session(self):
        """Delete the session and all associated data."""
        if self.session_id:
            try:
                requests.delete(f"{self.base_url}/api/sessions/{self.session_id}")
            except Exception as e:
                print(f"Warning: Failed to cleanup session {self.session_id}: {e}")
            finally:
                self.session_id = None


class SimulatorClient:
    """HTTP client wrapper for making requests to simulators."""

    def __init__(self, session_id, base_url=BASE_URL):
        self.session_id = session_id
        self.base_url = base_url
        self.headers = {"X-Session-ID": session_id}

    def get(self, path, params=None):
        """Make a GET request to the simulator."""
        url = f"{self.base_url}{path}"
        response = requests.get(url, headers=self.headers, params=params)
        return response

    def post(self, path, json=None, data=None):
        """Make a POST request to the simulator."""
        url = f"{self.base_url}{path}"
        response = requests.post(url, headers=self.headers, json=json, data=data)
        return response

    def put(self, path, json=None):
        """Make a PUT request to the simulator."""
        url = f"{self.base_url}{path}"
        response = requests.put(url, headers=self.headers, json=json)
        return response

    def delete(self, path):
        """Make a DELETE request to the simulator."""
        url = f"{self.base_url}{path}"
        response = requests.delete(url, headers=self.headers)
        return response
