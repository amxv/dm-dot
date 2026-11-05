package server

import (
	"context"
	"errors"
	"fmt"

	"github.com/amxv/dm-dot/pkg/message"
)

var (
	ErrInvalidMessage       = message.ErrInvalidMessage
	ErrNoActiveSubscription = errors.New("no active dot subscription")
	ErrSubscriptionStorage  = errors.New("subscription storage unavailable")
	ErrWebhookDelivery      = errors.New("webhook delivery failed")
)

// SendMessage is shared by the MCP tool and the REST API. A nil error only
// means the ChatGPT webhook accepted the event, not that the dot processed it.
func SendMessage(ctx context.Context, input string) (string, error) {
	normalized, err := message.Normalize(input)
	if err != nil {
		return "", err
	}

	sub, err := (subscriptionStore{}).get(ctx)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrSubscriptionStorage, err)
	}
	if sub == nil {
		return "", ErrNoActiveSubscription
	}

	eventID, err := deliverMessage(ctx, *sub, normalized)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrWebhookDelivery, err)
	}
	return eventID, nil
}
