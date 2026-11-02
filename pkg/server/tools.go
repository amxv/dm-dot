package server

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Exactly one public tool. There is exactly one dot; no dot ID or routing
// argument is accepted, and no reply channel is exposed.
func makeServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "dm-dot", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "send_to_dot",
		Description: "Deliver a message to the one subscribed ChatGPT dot. One-way only; webhook acceptance does not mean the dot has finished processing it.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, input struct {
		Message string `json:"message" jsonschema:"Message to send to the subscribed dot"`
	}) (*mcp.CallToolResult, any, error) {
		message := strings.TrimSpace(input.Message)
		if len(message) == 0 || len(message) > 16*1024 {
			return toolError("message must contain between 1 and 16384 UTF-8 bytes"), nil, nil
		}
		subscription, err := subscriptionStore{}.get(ctx)
		if err != nil {
			return toolError("Cannot load the dot subscription: " + err.Error()), nil, nil
		}
		if subscription == nil || time.Now().After(subscription.ExpiresAt) {
			return toolError("No active dot subscription. Have the dot subscribe to message.created first."), nil, nil
		}
		eventID, err := deliverMessage(ctx, *subscription, message)
		if err != nil {
			return toolError("ChatGPT did not acknowledge delivery: " + err.Error()), nil, nil
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("ChatGPT accepted event %s for asynchronous processing by your dot.", eventID)}}}, nil, nil
	})
	return s
}

func toolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}
