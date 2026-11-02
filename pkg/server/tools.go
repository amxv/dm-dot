package server

import (
	"context"
	"errors"
	"fmt"

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
		eventID, err := SendMessage(ctx, input.Message)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidMessage):
				return toolError(ErrInvalidMessage.Error()), nil, nil
			case errors.Is(err, ErrNoActiveSubscription):
				return toolError("No active dot subscription. Have the dot subscribe to message.created first."), nil, nil
			case errors.Is(err, ErrSubscriptionStorage):
				// Underlying Redis errors can include endpoint URLs or other private details.
				return toolError("Cannot load the dot subscription. Check the server configuration and try again."), nil, nil
			default:
				// HTTP client errors can contain the full webhook URL (including query secrets).
				return toolError("ChatGPT did not acknowledge delivery. Try again later."), nil, nil
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf("ChatGPT accepted event %s for asynchronous processing by your dot.", eventID)}}}, nil, nil
	})
	return s
}

func toolError(message string) *mcp.CallToolResult {
	return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: message}}}
}
