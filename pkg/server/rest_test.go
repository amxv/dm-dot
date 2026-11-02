package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func restRequest(t *testing.T, method, body, token, contentType string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/v1/messages", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	recorder := httptest.NewRecorder()
	RESTHandler().ServeHTTP(recorder, req)
	return recorder
}

func TestRESTAuthAndValidation(t *testing.T) {
	t.Setenv("DM_DOT_REST_API_KEY", "")
	if got := restRequest(t, http.MethodPost, `{}`, "anything", "application/json").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("missing config = HTTP %d, want 503", got)
	}

	t.Setenv("DM_DOT_REST_API_KEY", "separate-rest-key")
	for _, tc := range []struct {
		name, method, body, token, contentType string
		want                                   int
	}{
		{"method", http.MethodGet, "", "", "", http.StatusMethodNotAllowed},
		{"missing token", http.MethodPost, `{}`, "", "application/json", http.StatusUnauthorized},
		{"incorrect token", http.MethodPost, `{}`, "wrong-key", "application/json", http.StatusUnauthorized},
		{"MCP key is not REST token", http.MethodPost, `{}`, "mcp-key", "application/json", http.StatusUnauthorized},
		{"content type", http.MethodPost, `{}`, "separate-rest-key", "text/plain", http.StatusUnsupportedMediaType},
		{"malformed JSON", http.MethodPost, `{`, "separate-rest-key", "application/json", http.StatusBadRequest},
		{"extra property", http.MethodPost, `{"message":"hello","recipient":"other"}`, "separate-rest-key", "application/json", http.StatusBadRequest},
		{"multiple JSON", http.MethodPost, `{"message":"a"}{"message":"b"}`, "separate-rest-key", "application/json", http.StatusBadRequest},
		{"empty message", http.MethodPost, `{"message":"  "}`, "separate-rest-key", "application/json", http.StatusBadRequest},
		{"too long", http.MethodPost, `{"message":"` + strings.Repeat("a", 16385) + `"}`, "separate-rest-key", "application/json", http.StatusBadRequest},
		{"body too big", http.MethodPost, `{"message":"` + strings.Repeat("a", 33*1024) + `"}`, "separate-rest-key", "application/json", http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := restRequest(t, tc.method, tc.body, tc.token, tc.contentType)
			if recorder.Code != tc.want {
				t.Errorf("HTTP %d, want %d: %s", recorder.Code, tc.want, recorder.Body.String())
			}
			if recorder.Header().Get("Cache-Control") != "no-store" {
				t.Error("cache-control should be no-store")
			}
		})
	}
}

func TestRESTNoActiveSubscription(t *testing.T) {
	t.Setenv("DM_DOT_REST_API_KEY", "rest-token")
	t.Setenv("UPSTASH_REDIS_REST_URL", "")
	if got := restRequest(t, http.MethodPost, `{"message":"hello"}`, "rest-token", "application/json").Code; got != http.StatusServiceUnavailable {
		t.Fatalf("missing Redis = HTTP %d, want 503", got)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake" {
			t.Error("missing Redis authorization")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": nil})
	}))
	defer server.Close()
	t.Setenv("UPSTASH_REDIS_REST_URL", server.URL)
	t.Setenv("UPSTASH_REDIS_REST_TOKEN", "fake")
	previous := newRedisClient
	newRedisClient = server.Client
	defer func() { newRedisClient = previous }()

	response := restRequest(t, http.MethodPost, `{"message":"hello"}`, "rest-token", "application/json; charset=utf-8")
	if response.Code != http.StatusConflict || !strings.Contains(response.Body.String(), `"no_subscription"`) {
		t.Fatalf("no subscription: HTTP %d, body %s", response.Code, response.Body.String())
	}
}

func TestRESTAcceptedEvent(t *testing.T) {
	t.Setenv("DM_DOT_REST_API_KEY", "rest-token")
	t.Setenv("UPSTASH_REDIS_REST_TOKEN", "fake")
	subscription := subscription{
		ID: "sub_test", URL: "https://example.com/webhook",
		Secret:    "whsec_MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=",
		ExpiresAt: time.Now().Add(time.Hour),
	}
	stored, err := json.Marshal(subscription)
	if err != nil {
		t.Fatal(err)
	}
	redis := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"result": string(stored)})
	}))
	defer redis.Close()
	t.Setenv("UPSTASH_REDIS_REST_URL", redis.URL)
	prevRedis := newRedisClient
	newRedisClient = redis.Client
	defer func() { newRedisClient = prevRedis }()
	prevCallback := newCallbackClient
	var delivered string
	newCallbackClient = func() *http.Client {
		return &http.Client{Transport: httpRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(r.Body)
			var payload struct {
				Data struct {
					Message string `json:"message"`
				} `json:"data"`
			}
			if err := json.Unmarshal(body, &payload); err != nil {
				t.Error(err)
			}
			delivered = payload.Data.Message
			return &http.Response{StatusCode: 202, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
	}
	defer func() { newCallbackClient = prevCallback }()

	recorder := restRequest(t, http.MethodPost, `{"message":"  Hello from REST!  "}`, "rest-token", "application/json")
	if recorder.Code != http.StatusAccepted {
		t.Fatalf("HTTP %d, want 202: %s", recorder.Code, recorder.Body.String())
	}
	var result struct {
		Status  string `json:"status"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Status != "accepted" || !strings.HasPrefix(result.EventID, "evt_") || delivered != "Hello from REST!" {
		t.Fatalf("invalid result %#v, delivered %q", result, delivered)
	}
}
