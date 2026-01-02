package main

import (
	"log"
	"net/http"

	"github.com/recreate-run/nova-simulators/pkg/database"
	"github.com/recreate-run/nova-simulators/pkg/logging"
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
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.Close()

	queries := database.GetQueries()

	// Create main router
	mux := http.NewServeMux()

	// Register Slack simulator with logging middleware
	slackHandler := logging.Middleware("slack")(slack.NewHandler(queries))
	mux.Handle("/slack/", http.StripPrefix("/slack", slackHandler))

	// Future simulators will be added here:
	// hubspotHandler := logging.Middleware("hubspot")(hubspot.NewHandler(db))
	// mux.Handle("/hubspot/", http.StripPrefix("/hubspot", hubspotHandler))

	log.Println("Nova Simulators starting on :9000")
	log.Println("  - Slack: http://localhost:9000/slack")
	log.Println("Logging to: simulator.log")
	log.Fatal(http.ListenAndServe(":9000", mux))
}
