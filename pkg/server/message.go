package server

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidMessage       = errors.New("message must contain between 1 and 16384 UTF-8 bytes")
	ErrNoActiveSubscription = errors.New("no active dot subscription")
	ErrSubscriptionStorage  = errors.New("subscription storage unavailable")
	ErrWebhookDelivery      = errors.New("webhook delivery failed")
)

// SendMessage is shared by the MCP tool and the REST API. A nil error only
// means the ChatGPT webhook accepted the event, not that the dot processed it.
func SendMessage(ctx context.Context, input string) (string, error) {
	message := strings.TrimSpace(input)
	if len(message) == 0 || len(message) > 16*1024 {
		return "", ErrInvalidMessage
	}

	sub, err := (subscriptionStore{}).get(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSubscriptionStorage, err)
	}
	if sub == nil {
		return "", ErrNoActiveSubscription
	}

	eventID, err := deliverMessage(ctx, *sub, message)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrWebhookDelivery, err)
	}
	return eventID, nil
}
