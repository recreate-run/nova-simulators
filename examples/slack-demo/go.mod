module slack-demo

go 1.22

require (
	github.com/recreate-run/nova-simulators v0.0.0
	github.com/slack-go/slack v0.17.3
)

require github.com/gorilla/websocket v1.5.3 // indirect

replace github.com/recreate-run/nova-simulators => ../..
