package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/amxv/dm-dot/pkg/message"
)

func TestSendAcceptance(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer secret" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request headers")
		}
		body, err := io.ReadAll(r.Body)
		if err != nil || string(body) != `{"message":"hello"}` {
			t.Errorf("unexpected body %q: %v", body, err)
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = io.WriteString(w, `{"status":"accepted","event_id":"evt_abc"}`)
	}))
	defer server.Close()
	for _, endpoint := range []string{server.URL, server.URL + "/", server.URL + "/v1/messages"} {
		result, err := (Client{APIURL: endpoint, Token: "secret", HTTPClient: server.Client()}).Send(context.Background(), "  hello \n")
		if err != nil || result.EventID != "evt_abc" || result.Status != "accepted" {
			t.Errorf("endpoint %q: %#v, %v", endpoint, result, err)
		}
	}
}

func TestSendFailures(t *testing.T) {
	for _, tc := range []struct {
		name, endpoint, token, input string
		want                         error
	}{
		{"missing URL", "", "token", "hello", ErrMissingURL},
		{"missing token", "https://example.com", "", "hello", ErrMissingToken},
		{"empty", "https://example.com", "token", "   ", message.ErrInvalidMessage},
		{"too long", "https://example.com", "token", strings.Repeat("a", message.MaxBytes+1), message.ErrInvalidMessage},
		{"http", "http://example.com", "token", "hello", ErrInvalidURL},
		{"credentials", "https://me:secret@example.com", "token", "hello", ErrInvalidURL},
		{"query", "https://example.com/?key=secret", "token", "hello", ErrInvalidURL},
		{"fragment", "https://example.com/#token", "token", "hello", ErrInvalidURL},
		{"path", "https://example.com/mcp", "token", "hello", ErrInvalidURL},
		{"escape", "https://example.com/%76%31/messages", "token", "hello", ErrInvalidURL},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (Client{APIURL: tc.endpoint, Token: tc.token}).Send(context.Background(), tc.input)
			if !errors.Is(err, tc.want) {
				t.Fatalf("got %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSendDoesNotFollowRedirectOrRetry(t *testing.T) {
	requests := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Location", "https://example.com/steal")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	_, err := (Client{APIURL: server.URL, Token: "secret", HTTPClient: server.Client()}).Send(context.Background(), "hello")
	var remote *RemoteError
	if !errors.As(err, &remote) || remote.StatusCode != 307 || requests != 1 {
		t.Fatalf("got %v with %d requests", err, requests)
	}
}

func TestSendAPIErrorAndMalformedAcceptance(t *testing.T) {
	status := http.StatusConflict
	response := `{"error":{"code":"no_subscription","message":"No active dot subscription"}}`
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = fmt.Fprint(w, response)
	}))
	defer server.Close()
	client := Client{APIURL: server.URL, Token: "secret", HTTPClient: server.Client()}
	_, err := client.Send(context.Background(), "hello")
	var remote *RemoteError
	if !errors.As(err, &remote) || remote.Code != "no_subscription" || remote.StatusCode != status {
		t.Fatalf("got %v", err)
	}
	status, response = http.StatusAccepted, `{"status":"pending"}`
	_, err = client.Send(context.Background(), "hello")
	if err == nil || !strings.Contains(err.Error(), "invalid acceptance") {
		t.Fatalf("got %v", err)
	}
}
