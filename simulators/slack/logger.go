package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var logFile *os.File

func initLogger() error {
	var err error
	logFile, err = os.OpenFile("simulator.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	return nil
}

func closeLogger() {
	if logFile != nil {
		logFile.Close()
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

func loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		next(capture, r)

		duration := time.Since(start)

		// Pretty-print response JSON
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, capture.body.Bytes(), "", "  "); err != nil {
			prettyJSON.WriteString(capture.body.String())
		}

		// Log to file
		logEntry := fmt.Sprintf(`
========================================
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
			start.Format("2006-01-02 15:04:05.000"),
			r.Method,
			r.URL.Path,
			duration,
			capture.statusCode,
			requestBody,
			prettyJSON.String(),
		)

		if logFile != nil {
			logFile.WriteString(logEntry)
		}
	}
}
