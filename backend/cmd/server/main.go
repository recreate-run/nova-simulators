package main

import (
	"log"
	"net/http"
	"time"

	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/logging"
	"github.com/recreate-run/nova-simulators/internal/session"
	"github.com/recreate-run/nova-simulators/simulators/gmail"
	"github.com/recreate-run/nova-simulators/simulators/slack"
)

func main() {
	// Initialize unified logger
	log.SetFlags(0)
	if err := logging.InitLogger("simulator.log"); err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logging.CloseLogger()

	// Initialize database
	if err := database.InitDB("file:simulators.db"); err != nil {
		logging.CloseLogger()
		log.Panicf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := database.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()

	queries := database.GetQueries()

	// Create main router
	mux := http.NewServeMux()

	// Register session manager (no session middleware needed for session mgmt endpoints)
	sessionManager := session.NewManager(queries)
	mux.Handle("/sessions", sessionManager)
	mux.Handle("/sessions/", sessionManager)

	// Register Slack simulator with session + logging middleware
	slackHandler := session.Middleware(logging.Middleware("slack")(slack.NewHandler(queries)))
	mux.Handle("/slack/", http.StripPrefix("/slack", slackHandler))

	// Register Gmail simulator with session + logging middleware
	gmailHandler := session.Middleware(logging.Middleware("gmail")(gmail.NewHandler(queries)))
	mux.Handle("/gmail/", http.StripPrefix("/gmail", gmailHandler))

	// Future simulators will be added here:
	// hubspotHandler := session.Middleware(logging.Middleware("hubspot")(hubspot.NewHandler(db)))
	// mux.Handle("/hubspot/", http.StripPrefix("/hubspot", hubspotHandler))

	log.Println("Nova Simulators starting on :9000")
	log.Println("  - Sessions: http://localhost:9000/sessions")
	log.Println("  - Slack: http://localhost:9000/slack")
	log.Println("  - Gmail: http://localhost:9000/gmail")
	log.Println("Logging to: simulator.log")

	// Create server with timeouts
	server := &http.Server{
		Addr:         ":9000",
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Fatal(server.ListenAndServe()) //nolint:gocritic // exitAfterDefer is acceptable here - server.ListenAndServe only returns on fatal error
}
