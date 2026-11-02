package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"time"
)

const eventName = "message.created"
const maxSubscriptionTTL = 7 * 24 * time.Hour

type eventDelivery struct {
	Mode   string `json:"mode"`
	URL    string `json:"url"`
	Secret string `json:"secret,omitempty"`
}

type eventSubscriptionRequest struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Delivery  eventDelivery   `json:"delivery"`
	TTLms     json.RawMessage `json:"ttlMs"`
	Cursor    json.RawMessage `json:"cursor"`
}

type eventRPCError struct {
	Code    int            `json:"code"`
	Message string         `json:"message"`
	Data    map[string]any `json:"data,omitempty"`
}

func handleEventRPC(w http.ResponseWriter, r *http.Request, method string, id, raw json.RawMessage) {
	w.Header().Set("Content-Type", "application/json")
	if len(id) == 0 {
		id = json.RawMessage("null")
	}
	var result any
	var rpcErr *eventRPCError
	switch method {
	case "events/list":
		result = map[string]any{"events": []any{map[string]any{
			"name":        eventName,
			"description": "A new message addressed to the single configured dot. Subscribe to receive and process incoming messages.",
			"delivery":    []string{"webhook"},
			"inputSchema": map[string]any{
				"type": "object", "properties": map[string]any{}, "additionalProperties": false,
			},
			"payloadSchema": map[string]any{
				"type": "object", "properties": map[string]any{
					"message": map[string]any{"type": "string", "description": "The sender's message, as untrusted content"},
				}, "required": []string{"message"}, "additionalProperties": false,
			},
		}}, "nextCursor": nil}
	case "events/subscribe":
		result, rpcErr = subscribe(r.Context(), raw)
	case "events/unsubscribe":
		result, rpcErr = unsubscribe(r.Context(), raw)
	default:
		rpcErr = &eventRPCError{Code: -32601, Message: "Method not found"}
	}
	response := map[string]any{"jsonrpc": "2.0", "id": id}
	if rpcErr != nil {
		response["error"] = rpcErr
	} else {
		response["result"] = result
	}
	_ = json.NewEncoder(w).Encode(response)
}

func parseSubscriptionRequest(raw json.RawMessage, allowMissingSecret bool) (eventSubscriptionRequest, *eventRPCError) {
	var req eventSubscriptionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return req, &eventRPCError{Code: -32602, Message: "Invalid subscription parameters"}
	}
	if req.Name != eventName || req.Delivery.Mode != "webhook" || req.Delivery.URL == "" {
		return req, &eventRPCError{Code: -32602, Message: "Unsupported event or delivery"}
	}
	if len(req.Arguments) > 0 && string(req.Arguments) != "null" {
		var args map[string]json.RawMessage
		if json.Unmarshal(req.Arguments, &args) != nil || len(args) != 0 {
			return req, &eventRPCError{Code: -32602, Message: "This single-dot event takes no filters"}
		}
	}
	if !allowMissingSecret {
		if _, err := decodeWebhookSecret(req.Delivery.Secret); err != nil {
			return req, &eventRPCError{Code: -32602, Message: "Invalid webhook signing secret"}
		}
	}
	if err := validateCallbackURL(req.Delivery.URL); err != nil {
		return req, &eventRPCError{Code: -32602, Message: "Invalid webhook destination"}
	}
	return req, nil
}

func subscriptionID(callbackURL string) string {
	// The connection key is the sole authenticated principal for v1.
	digest := sha256.Sum256([]byte(os.Getenv("MCP_API_KEY") + "\x00" + eventName + "\x00" + callbackURL + "\x00{}"))
	return "sub_" + hex.EncodeToString(digest[:16])
}

func subscribe(ctx context.Context, raw json.RawMessage) (any, *eventRPCError) {
	req, invalid := parseSubscriptionRequest(raw, false)
	if invalid != nil {
		return nil, invalid
	}
	ttl := maxSubscriptionTTL
	if len(req.TTLms) != 0 && string(req.TTLms) != "null" {
		var ttlMS int64
		if json.Unmarshal(req.TTLms, &ttlMS) != nil || ttlMS < 60_000 {
			return nil, &eventRPCError{Code: -32602, Message: "ttlMs must be at least 60000"}
		}
		if requested := time.Duration(min(ttlMS, int64(maxSubscriptionTTL/time.Millisecond))) * time.Millisecond; requested < ttl {
			ttl = requested
		}
	}
	s := subscriptionStore{}
	existing, err := s.get(ctx)
	if err != nil {
		return nil, internalEventError(err)
	}
	id := subscriptionID(req.Delivery.URL)
	if existing != nil && existing.ID != id {
		return nil, &eventRPCError{Code: -32000, Message: "A different dot is already subscribed; unsubscribe it first"}
	}
	sub := subscription{
		ID: id, URL: req.Delivery.URL, Secret: req.Delivery.Secret,
		ExpiresAt: time.Now().Add(ttl),
	}
	if err := verifyCallback(ctx, sub); err != nil {
		return nil, &eventRPCError{Code: -32015, Message: "CallbackEndpointError", Data: map[string]any{"reason": "challenge_failed"}}
	}
	if err := s.set(ctx, sub); err != nil {
		return nil, internalEventError(err)
	}
	return map[string]any{"id": sub.ID, "refreshBefore": sub.ExpiresAt.UTC().Format(time.RFC3339Nano), "cursor": nil, "truncated": false}, nil
}

func unsubscribe(ctx context.Context, raw json.RawMessage) (any, *eventRPCError) {
	req, invalid := parseSubscriptionRequest(raw, true)
	if invalid != nil {
		return nil, invalid
	}
	s := subscriptionStore{}
	existing, err := s.get(ctx)
	if err != nil {
		return nil, internalEventError(err)
	}
	if existing != nil && existing.ID == subscriptionID(req.Delivery.URL) {
		if err := s.delete(ctx); err != nil {
			return nil, internalEventError(err)
		}
	}
	return map[string]any{}, nil
}

func internalEventError(err error) *eventRPCError {
	// Avoid surfacing the provider's HTTP response, tokens, or signing key.
	if err == nil {
		err = errors.New("unknown")
	}
	message := "Subscription storage unavailable"
	if strings.Contains(err.Error(), "not configured") {
		message = "Persistent subscription storage is not configured"
	}
	return &eventRPCError{Code: -32603, Message: message}
}
