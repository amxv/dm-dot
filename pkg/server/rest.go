package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/amxv/dm-dot/pkg/message"
)

// RESTHandler accepts server-to-server requests using a REST-specific bearer
// token. It never accepts the MCP query-string key as authentication.
func RESTHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Robots-Tag", "noindex")
		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		if r.Method != http.MethodPost {
			w.Header().Set("Allow", "POST")
			restError(w, http.StatusMethodNotAllowed, "method_not_allowed", "Use POST /v1/messages")
			return
		}

		secret := os.Getenv("DM_DOT_REST_API_KEY")
		if strings.TrimSpace(secret) == "" {
			restError(w, http.StatusServiceUnavailable, "not_configured", "REST API is not configured")
			return
		}
		auth := r.Header.Values("Authorization")
		if len(auth) != 1 || !validBearer(secret, auth[0]) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="dm-dot"`)
			restError(w, http.StatusUnauthorized, "unauthorized", "Invalid or missing bearer token")
			return
		}

		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			restError(w, http.StatusUnsupportedMediaType, "unsupported_media_type", "Content-Type must be application/json")
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, message.MaxRESTBodyBytes+1))
		if err != nil {
			restError(w, http.StatusBadRequest, "invalid_body", "Could not read request body")
			return
		}
		if len(body) > message.MaxRESTBodyBytes {
			restError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "JSON request is too large")
			return
		}
		var payload message.Request
		decoder := json.NewDecoder(strings.NewReader(string(body)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&payload); err != nil {
			restError(w, http.StatusBadRequest, "invalid_json", "Expected JSON object with a message string")
			return
		}
		if decoder.Decode(&struct{}{}) != io.EOF {
			restError(w, http.StatusBadRequest, "invalid_json", "Only one JSON object is allowed")
			return
		}

		eventID, err := SendMessage(r.Context(), payload.Message)
		if err != nil {
			switch {
			case errors.Is(err, ErrInvalidMessage):
				restError(w, http.StatusBadRequest, "invalid_message", ErrInvalidMessage.Error())
			case errors.Is(err, ErrNoActiveSubscription):
				restError(w, http.StatusConflict, "no_subscription", "No active dot subscription")
			case errors.Is(err, ErrSubscriptionStorage):
				restError(w, http.StatusServiceUnavailable, "storage_unavailable", "Subscription storage unavailable")
			default:
				restError(w, http.StatusBadGateway, "delivery_failed", "ChatGPT did not acknowledge webhook delivery")
			}
			return
		}

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(message.Accepted{Status: "accepted", EventID: eventID})
	})
}

func validBearer(expected, header string) bool {
	parts := strings.Fields(header)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
		return false
	}
	want := sha256.Sum256([]byte(expected))
	got := sha256.Sum256([]byte(parts[1]))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func restError(w http.ResponseWriter, status int, code, message string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{
		"code": code, "message": message,
	}})
}
