package server

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func mcpToolErrorText(t *testing.T) string {
	t.Helper()
	response := rpc(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send_to_dot","arguments":{"message":"test message"}}}`)
	result, ok := response["result"].(map[string]any)
	if !ok || result["isError"] != true {
		t.Fatalf("expected MCP tool error, got: %v", response)
	}
	content := result["content"].([]any)
	return content[0].(map[string]any)["text"].(string)
}

func TestMCPToolStorageErrorDoesNotExposeInfrastructure(t *testing.T) {
	t.Setenv("MCP_API_KEY", "test-key")
	t.Setenv("UPSTASH_REDIS_REST_URL", "https://redis.example.com/private?token=redis-secret")
	t.Setenv("UPSTASH_REDIS_REST_TOKEN", "redis-secret")

	previous := newRedisClient
	newRedisClient = func() *http.Client {
		return &http.Client{Transport: httpRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("private Redis error: redis-secret")
		})}
	}
	defer func() { newRedisClient = previous }()

	message := mcpToolErrorText(t)
	if !strings.Contains(message, "Cannot load the dot subscription") {
		t.Fatalf("unexpected storage error message: %q", message)
	}
	for _, private := range []string{"redis.example.com", "redis-secret", "private Redis error"} {
		if strings.Contains(message, private) {
			t.Errorf("storage error revealed private detail %q: %q", private, message)
		}
	}
}

func TestMCPToolDeliveryErrorDoesNotExposeCallback(t *testing.T) {
	t.Setenv("MCP_API_KEY", "test-key")
	t.Setenv("UPSTASH_REDIS_REST_URL", "https://redis.example.com")
	t.Setenv("UPSTASH_REDIS_REST_TOKEN", "test-redis-key")

	subscription := subscription{
		ID:        "sub_test",
		URL:       "https://receiver.example.com/private?token=webhook-secret",
		Secret:    "whsec_" + base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901")),
		ExpiresAt: time.Now().Add(time.Hour),
	}
	stored, err := json.Marshal(subscription)
	if err != nil {
		t.Fatal(err)
	}
	previousRedis := newRedisClient
	newRedisClient = func() *http.Client {
		return &http.Client{Transport: httpRoundTripFunc(func(*http.Request) (*http.Response, error) {
			body, _ := json.Marshal(map[string]any{"result": string(stored)})
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}, nil
		})}
	}
	defer func() { newRedisClient = previousRedis }()

	previousCallback := newCallbackClient
	newCallbackClient = func() *http.Client {
		return &http.Client{Transport: httpRoundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("private callback error: webhook-secret")
		})}
	}
	defer func() { newCallbackClient = previousCallback }()

	message := mcpToolErrorText(t)
	if !strings.Contains(message, "ChatGPT did not acknowledge delivery") {
		t.Fatalf("unexpected delivery error message: %q", message)
	}
	for _, private := range []string{"receiver.example.com", "webhook-secret", "private callback error", "?token="} {
		if strings.Contains(message, private) {
			t.Errorf("delivery error revealed private detail %q: %q", private, message)
		}
	}
}
