package main

import (
	"log"
	"net/http"
	"os"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
	"github.com/recreate-run/nova-simulators/internal/database"
	"github.com/recreate-run/nova-simulators/internal/logging"
	"github.com/recreate-run/nova-simulators/internal/session"
	"github.com/recreate-run/nova-simulators/simulators/datadog"
	"github.com/recreate-run/nova-simulators/simulators/gdocs"
	githubsim "github.com/recreate-run/nova-simulators/simulators/github"
	"github.com/recreate-run/nova-simulators/simulators/gmail"
	"github.com/recreate-run/nova-simulators/simulators/gsheets"
	"github.com/recreate-run/nova-simulators/simulators/hubspot"
	"github.com/recreate-run/nova-simulators/simulators/jira"
	"github.com/recreate-run/nova-simulators/simulators/linear"
	"github.com/recreate-run/nova-simulators/simulators/outlook"
	"github.com/recreate-run/nova-simulators/simulators/pagerduty"
	postgressim "github.com/recreate-run/nova-simulators/simulators/postgres"
	"github.com/recreate-run/nova-simulators/simulators/resend"
	"github.com/recreate-run/nova-simulators/simulators/slack"
	"github.com/recreate-run/nova-simulators/simulators/whatsapp"
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

	// Start embedded PostgreSQL for Postgres simulator
	var embeddedPG *embeddedpostgres.EmbeddedPostgres
	var postgresHandler *postgressim.Handler

	// Only start embedded Postgres if POSTGRES_SIMULATOR_ENABLED is set
	if os.Getenv("POSTGRES_SIMULATOR_ENABLED") != "false" {
		embeddedPG = embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
			Port(5433).
			Database("simulator").
			Username("postgres").
			Password("postgres"))

		if err := embeddedPG.Start(); err != nil {
			log.Printf("Warning: Failed to start embedded Postgres: %v", err)
			log.Println("Postgres simulator will not be available")
		} else {
			defer func() {
				if err := embeddedPG.Stop(); err != nil {
					log.Printf("Failed to stop embedded Postgres: %v", err)
				}
			}()

			// Initialize Postgres handler
			pgConnStr := "host=localhost port=5433 user=postgres password=postgres dbname=simulator sslmode=disable"
			var err error
			postgresHandler, err = postgressim.NewHandler(queries, pgConnStr)
			if err != nil {
				log.Printf("Warning: Failed to initialize Postgres handler: %v", err)
				postgresHandler = nil
			} else {
				defer func() {
					if err := postgresHandler.Close(); err != nil {
						log.Printf("Failed to close Postgres handler: %v", err)
					}
				}()
			}
		}
	}

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

	// Register Google Docs simulator with session + logging middleware
	gdocsHandler := session.Middleware(logging.Middleware("gdocs")(gdocs.NewHandler(queries)))
	mux.Handle("/gdocs/", http.StripPrefix("/gdocs", gdocsHandler))

	// Register Google Sheets simulator with session + logging middleware
	gsheetsHandler := session.Middleware(logging.Middleware("gsheets")(gsheets.NewHandler(queries)))
	mux.Handle("/gsheets/", http.StripPrefix("/gsheets", gsheetsHandler))

	// Register Datadog simulator with session + logging middleware
	datadogHandler := session.Middleware(logging.Middleware("datadog")(datadog.NewHandler(queries)))
	mux.Handle("/datadog/", http.StripPrefix("/datadog", datadogHandler))

	// Register Resend simulator with session + logging middleware
	resendHandler := session.Middleware(logging.Middleware("resend")(resend.NewHandler(queries)))
	mux.Handle("/resend/", http.StripPrefix("/resend", resendHandler))

	// Register Linear simulator with session + logging middleware
	linearHandler := session.Middleware(logging.Middleware("linear")(linear.NewHandler(queries)))
	mux.Handle("/linear/", http.StripPrefix("/linear", linearHandler))

	// Register GitHub simulator with session + logging middleware
	githubHandler := session.Middleware(logging.Middleware("github")(githubsim.NewHandler(queries)))
	mux.Handle("/github/", http.StripPrefix("/github", githubHandler))

	// Register Outlook simulator with session + logging middleware
	outlookHandler := session.Middleware(logging.Middleware("outlook")(outlook.NewHandler(queries)))
	mux.Handle("/outlook/", http.StripPrefix("/outlook", outlookHandler))

	// Register PagerDuty simulator with session + logging middleware
	pagerdutyHandler := session.Middleware(logging.Middleware("pagerduty")(pagerduty.NewHandler(queries)))
	mux.Handle("/pagerduty/", http.StripPrefix("/pagerduty", pagerdutyHandler))

	// Register HubSpot simulator with session + logging middleware
	hubspotHandler := session.Middleware(logging.Middleware("hubspot")(hubspot.NewHandler(queries)))
	mux.Handle("/hubspot/", http.StripPrefix("/hubspot", hubspotHandler))

	// Register Jira simulator with session + logging middleware
	jiraHandler := session.Middleware(logging.Middleware("jira")(jira.NewHandler(queries)))
	mux.Handle("/jira/", http.StripPrefix("/jira", jiraHandler))

	// Register WhatsApp simulator with session + logging middleware
	whatsappHandler := session.Middleware(logging.Middleware("whatsapp")(whatsapp.NewHandler(queries)))
	mux.Handle("/whatsapp/", http.StripPrefix("/whatsapp", whatsappHandler))

	// Register Postgres simulator with session + logging middleware (if enabled)
	if postgresHandler != nil {
		pgHandler := session.Middleware(logging.Middleware("postgres")(postgresHandler))
		mux.Handle("/postgres/", http.StripPrefix("/postgres", pgHandler))
	}

	if postgresHandler != nil {
		log.Println("  - Postgres: http://localhost:9000/postgres (DB: localhost:5433)")
	}
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
