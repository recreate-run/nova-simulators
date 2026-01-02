package logging

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

var (
	logFile *os.File
	logMu   sync.Mutex
)

// InitLogger initializes the unified log file
func InitLogger(filename string) error {
	logMu.Lock()
	defer logMu.Unlock()

	var err error
	// #nosec G304 -- filename is controlled by the application, not user input
	logFile, err = os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	return nil
}

// CloseLogger closes the log file
func CloseLogger() {
	logMu.Lock()
	defer logMu.Unlock()

	if logFile != nil {
		_ = logFile.Close()
	}
}

type responseCapture struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rc *responseCapture) Write(b []byte) (int, error) {
	rc.body.Write(b)
	return rc.ResponseWriter.Write(b)
}

func (rc *responseCapture) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
	rc.ResponseWriter.WriteHeader(statusCode)
}

// Middleware creates a logging middleware for a specific simulator
func Middleware(simulatorName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			// Read and restore request body
			var requestBody string
			if r.Body != nil {
				bodyBytes, _ := io.ReadAll(r.Body)
				requestBody = string(bodyBytes)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// Capture response
			capture := &responseCapture{
				ResponseWriter: w,
				statusCode:     200,
				body:           &bytes.Buffer{},
			}

			// Process request
			next.ServeHTTP(capture, r)

			duration := time.Since(start)

			// Pretty-print response JSON
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, capture.body.Bytes(), "", "  "); err != nil {
				prettyJSON.WriteString(capture.body.String())
			}

			// Log to file
			logEntry := fmt.Sprintf(`
========================================
Simulator: %s
Time: %s
Method: %s %s
Duration: %v
Status: %d

Request Body (form-encoded):
%s

Response Body (JSON):
%s
========================================
`,
				simulatorName,
				start.Format("2006-01-02 15:04:05.000"),
				r.Method,
				r.URL.Path,
				duration,
				capture.statusCode,
				requestBody,
				prettyJSON.String(),
			)

			logMu.Lock()
			if logFile != nil {
				_, _ = logFile.WriteString(logEntry)
			}
			logMu.Unlock()
		})
	}
}
