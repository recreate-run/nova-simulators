package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/recreate-run/nova-simulators/pkg/transport"
	"github.com/slack-go/slack"
)

func main() {
	log.Println("=== Slack Sandbox Demo ===")
	log.Println()

	// Step 1: Install HTTP interceptor
	log.Println("1. Installing HTTP interceptor...")
	http.DefaultTransport = transport.NewSimulatorTransport(map[string]string{
		"slack.com": "localhost:9001",
	})
	log.Println("   ✓ Interceptor installed (slack.com → localhost:9001)")
	log.Println()

	// Step 2: Create Slack client using standard library
	log.Println("2. Creating Slack client...")
	client := slack.New("fake-token-12345")
	log.Println("   ✓ Client created")
	log.Println()

	// Step 3: Post a message
	log.Println("3. Posting message to #general...")
	_, timestamp, err := client.PostMessage("C001", slack.MsgOptionText("Hello from nova-simulators!", false))
	if err != nil {
		log.Fatalf("   ✗ Failed to post message: %v", err)
	}
	log.Printf("   ✓ Message posted (timestamp: %s)", timestamp)
	log.Println()

	// Step 4: Get channels
	log.Println("4. Getting channel list...")
	channels, _, err := client.GetConversations(&slack.GetConversationsParameters{
		ExcludeArchived: true,
		Types:           []string{"public_channel", "private_channel"},
	})
	if err != nil {
		log.Fatalf("   ✗ Failed to get channels: %v", err)
	}
	log.Printf("   ✓ Retrieved %d channels:", len(channels))
	for _, ch := range channels {
		log.Printf("      - %s (%s)", ch.Name, ch.ID)
	}
	log.Println()

	// Step 5: Get message history
	log.Println("5. Getting message history from #general...")
	history, err := client.GetConversationHistory(&slack.GetConversationHistoryParameters{
		ChannelID: "C001",
		Limit:     10,
	})
	if err != nil {
		log.Fatalf("   ✗ Failed to get history: %v", err)
	}
	log.Printf("   ✓ Retrieved %d messages:", len(history.Messages))
	for i, msg := range history.Messages {
		log.Printf("      %d. [%s] %s", i+1, msg.Timestamp, msg.Text)
	}
	log.Println()

	// Success!
	fmt.Println("=== Demo Complete ===")
	fmt.Println("✓ Existing Slack library works unchanged")
	fmt.Println("✓ HTTP requests intercepted and routed to simulator")
	fmt.Println("✓ Check simulators/slack/simulator.log for full request/response logs")
}
