package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func rpc(t *testing.T, body string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/mcp?key=test-key", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2025-11-25")
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unexpected HTTP %d: %s", rec.Code, rec.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("invalid JSON-RPC response: %v: %s", err, rec.Body.String())
	}
	return result
}

func TestAuthentication(t *testing.T) {
	t.Setenv("MCP_API_KEY", "")
	req := httptest.NewRequest(http.MethodPost, "/mcp?key=test-key", nil)
	rec := httptest.NewRecorder()
	Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("unconfigured endpoint: got HTTP %d", rec.Code)
	}
	t.Setenv("MCP_API_KEY", "test-key")
	for _, route := range []string{"/mcp", "/mcp?key=wrong", "/mcp?key=test-key&key=wrong"} {
		req := httptest.NewRequest(http.MethodPost, route, nil)
		rec := httptest.NewRecorder()
		Handler().ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("route %q: HTTP %d", route, rec.Code)
		}
	}
}

func TestInitializationAdvertisesEventsAndOnlyOneTool(t *testing.T) {
	t.Setenv("MCP_API_KEY", "test-key")
	init := rpc(t, `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`)
	result := init["result"].(map[string]any)
	if result["serverInfo"].(map[string]any)["name"] != "dm-dot" {
		t.Fatalf("unexpected serverInfo: %v", result["serverInfo"])
	}
	if _, ok := result["capabilities"].(map[string]any)["events"]; !ok {
		t.Fatalf("events capability missing: %v", result["capabilities"])
	}
	listed := rpc(t, `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}`)
	tools := listed["result"].(map[string]any)["tools"].([]any)
	if len(tools) != 1 || tools[0].(map[string]any)["name"] != "send_to_dot" {
		t.Fatalf("unexpected tools: %v", tools)
	}
	annotations := tools[0].(map[string]any)["annotations"].(map[string]any)
	if annotations["readOnlyHint"] != true {
		t.Fatalf("expected readOnlyHint: %v", annotations)
	}
	if _, present := annotations["openWorldHint"]; present {
		t.Fatalf("unexpected openWorldHint: %v", annotations)
	}
	events := rpc(t, `{"jsonrpc":"2.0","id":3,"method":"events/list","params":{}}`)
	descriptions := events["result"].(map[string]any)["events"].([]any)
	if len(descriptions) != 1 || descriptions[0].(map[string]any)["name"] != eventName {
		t.Fatalf("unexpected events: %v", descriptions)
	}
}

func TestMCP2DiscoverAdvertisesEvents(t *testing.T) {
	t.Setenv("MCP_API_KEY", "test-key")
	// MCP 2.0 requires the method header as well as per-request metadata.
	// ChatGPT can discover the server through this path instead of initialize.
	req := httptest.NewRequest(http.MethodPost, "/mcp?key=test-key", strings.NewReader(`{
		"jsonrpc":"2.0", "id":27, "method":"server/discover",
		"params":{"_meta":{
			"io.modelcontextprotocol/protocolVersion":"2026-07-28",
			"io.modelcontextprotocol/clientInfo":{"name":"dm-dot-event-discovery-test","version":"1.0.0"},
			"io.modelcontextprotocol/clientCapabilities":{}
		}}
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	req.Header.Set("Mcp-Method", "server/discover")
	recorder := httptest.NewRecorder()
	Handler().ServeHTTP(recorder, req)
	if recorder.Code != http.StatusOK {
		t.Fatalf("MCP 2.0 discovery HTTP %d: %s", recorder.Code, recorder.Body.String())
	}
	var response struct {
		Result struct {
			ResultType        string                     `json:"resultType"`
			SupportedVersions []string                   `json:"supportedVersions"`
			Capabilities      map[string]json.RawMessage `json:"capabilities"`
		} `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal MCP 2.0 discovery: %v", err)
	}
	if response.Result.ResultType != "complete" {
		t.Errorf("discovery resultType = %q", response.Result.ResultType)
	}
	if string(response.Result.Capabilities["events"]) != "{}" {
		t.Fatalf("MCP 2.0 events capability missing: %s", recorder.Body.String())
	}
	if _, ok := response.Result.Capabilities["tools"]; !ok {
		t.Fatalf("lost SDK tools capability: %s", recorder.Body.String())
	}
	var hasMCP2 bool
	for _, version := range response.Result.SupportedVersions {
		if version == "2026-07-28" {
			hasMCP2 = true
		}
	}
	if !hasMCP2 {
		t.Fatalf("lost MCP 2.0 supportedVersions: %v", response.Result.SupportedVersions)
	}

	// Verify the event catalog is retrievable with MCP 2.0 metadata.
	eventReq := httptest.NewRequest(http.MethodPost, "/mcp?key=test-key", strings.NewReader(`{
		"jsonrpc":"2.0", "id":28, "method":"events/list",
		"params":{"_meta":{
			"io.modelcontextprotocol/protocolVersion":"2026-07-28",
			"io.modelcontextprotocol/clientCapabilities":{}
		}}
	}`))
	eventReq.Header.Set("Content-Type", "application/json")
	eventReq.Header.Set("Accept", "application/json, text/event-stream")
	eventReq.Header.Set("Mcp-Protocol-Version", "2026-07-28")
	eventReq.Header.Set("Mcp-Method", "events/list")
	eventRecorder := httptest.NewRecorder()
	Handler().ServeHTTP(eventRecorder, eventReq)
	if eventRecorder.Code != http.StatusOK {
		t.Fatalf("MCP 2.0 events/list HTTP %d: %s", eventRecorder.Code, eventRecorder.Body.String())
	}
	var eventResponse struct {
		Result struct {
			Events []struct {
				Name     string   `json:"name"`
				Delivery []string `json:"delivery"`
			} `json:"events"`
		} `json:"result"`
	}
	if err := json.Unmarshal(eventRecorder.Body.Bytes(), &eventResponse); err != nil {
		t.Fatalf("unmarshal events/list: %v", err)
	}
	if len(eventResponse.Result.Events) != 1 || eventResponse.Result.Events[0].Name != eventName || len(eventResponse.Result.Events[0].Delivery) != 1 || eventResponse.Result.Events[0].Delivery[0] != "webhook" {
		t.Fatalf("unexpected MCP 2.0 events/list: %s", eventRecorder.Body.String())
	}
}

func TestSendRequiresSubscriptionAndValidMessage(t *testing.T) {
	t.Setenv("MCP_API_KEY", "test-key")
	t.Setenv("UPSTASH_REDIS_REST_URL", "")
	result := rpc(t, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"send_to_dot","arguments":{"message":"hi"}}}`)
	call := result["result"].(map[string]any)
	if call["isError"] != true {
		t.Fatalf("unconfigured storage must fail: %v", call)
	}
	result = rpc(t, `{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"send_to_dot","arguments":{"message":" "}}}`)
	call = result["result"].(map[string]any)
	if call["isError"] != true {
		t.Fatalf("empty message must fail: %v", call)
	}
}

type httpRoundTripFunc func(*http.Request) (*http.Response, error)

func (f httpRoundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func parseUnix(t *testing.T, value string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		t.Fatalf("invalid signing timestamp: %v", err)
	}
	return n
}

func TestSubscribeSendRefreshAndUnsubscribe(t *testing.T) {
	t.Setenv("MCP_API_KEY", "test-key")
	t.Setenv("UPSTASH_REDIS_REST_TOKEN", "fake-redis-token")
	var mu sync.Mutex
	var record string
	redis := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fake-redis-token" {
			t.Error("missing Redis authorization")
		}
		var args []any
		if json.NewDecoder(r.Body).Decode(&args) != nil {
			t.Error("bad Redis command")
			return
		}
		mu.Lock()
		defer mu.Unlock()
		var value any
		switch args[0] {
		case "GET":
			if record != "" {
				value = record
			}
		case "SET":
			record = args[2].(string)
			value = "OK"
		case "DEL":
			record = ""
			value = 1
		default:
			t.Errorf("unexpected Redis command: %v", args)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"result": value})
	}))
	defer redis.Close()
	t.Setenv("UPSTASH_REDIS_REST_URL", redis.URL)
	oldRedisClient := newRedisClient
	newRedisClient = redis.Client
	defer func() { newRedisClient = oldRedisClient }()

	key := base64.StdEncoding.EncodeToString([]byte("01234567890123456789012345678901"))
	secret := "whsec_" + key
	var delivered []string
	oldCallbackClient := newCallbackClient
	newCallbackClient = func() *http.Client {
		return &http.Client{Transport: httpRoundTripFunc(func(r *http.Request) (*http.Response, error) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				t.Errorf("read webhook: %v", err)
			}
			signed, err := signedWebhook(secret, r.Header.Get("webhook-id"), parseUnix(t, r.Header.Get("webhook-timestamp")), body)
			if err != nil || signed != r.Header.Get("webhook-signature") {
				t.Errorf("invalid webhook signature: %v", err)
			}
			if r.Header.Get("X-MCP-Subscription-Id") == "" {
				t.Error("missing subscription ID")
			}
			var payload map[string]any
			if json.Unmarshal(body, &payload) != nil {
				t.Error("invalid callback JSON")
			}
			if payload["type"] == "verification" {
				out, _ := json.Marshal(map[string]any{"challenge": payload["challenge"]})
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(out))), Header: make(http.Header)}, nil
			}
			delivered = append(delivered, payload["data"].(map[string]any)["message"].(string))
			return &http.Response{StatusCode: 204, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
		})}
	}
	defer func() { newCallbackClient = oldCallbackClient }()

	params, _ := json.Marshal(map[string]any{"name": eventName, "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.com/callback", "secret": secret}})
	subscribeMessage := `{"jsonrpc":"2.0","id":1,"method":"events/subscribe","params":` + string(params) + `}`
	first := rpc(t, subscribeMessage)
	sub := first["result"].(map[string]any)
	if sub["id"] == "" || sub["refreshBefore"] == "" {
		t.Fatalf("missing subscription fields: %v", sub)
	}
	second := rpc(t, subscribeMessage)
	if second["result"].(map[string]any)["id"] != sub["id"] {
		t.Fatal("refresh changed subscription ID")
	}

	call := rpc(t, `{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"send_to_dot","arguments":{"message":"hello dot"}}}`)
	if call["result"].(map[string]any)["isError"] == true {
		t.Fatalf("send failed: %v", call)
	}
	if len(delivered) != 1 || delivered[0] != "hello dot" {
		t.Fatalf("unexpected webhook messages: %v", delivered)
	}

	otherParams, _ := json.Marshal(map[string]any{"name": eventName, "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://another.example.com/callback", "secret": secret}})
	other := rpc(t, `{"jsonrpc":"2.0","id":4,"method":"events/subscribe","params":`+string(otherParams)+`}`)
	if other["error"] == nil {
		t.Fatal("another dot should not replace existing subscription")
	}
	unsubParams, _ := json.Marshal(map[string]any{"name": eventName, "arguments": map[string]any{}, "delivery": map[string]any{"mode": "webhook", "url": "https://receiver.example.com/callback"}})
	unsub := rpc(t, `{"jsonrpc":"2.0","id":5,"method":"events/unsubscribe","params":`+string(unsubParams)+`}`)
	if unsub["error"] != nil {
		t.Fatalf("unsubscribe error: %v", unsub)
	}
	call = rpc(t, `{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"send_to_dot","arguments":{"message":"should fail"}}}`)
	if call["result"].(map[string]any)["isError"] != true {
		t.Fatal("send after unsubscribe should fail")
	}
}

func TestValidateCallbackAndWebhookSecret(t *testing.T) {
	for _, raw := range []string{"http://example.com/event", "https://localhost/event", "https://127.0.0.1/event", "https://192.168.1.1/event", "https://example.com:8080/event", "https://user:pw@example.com/event"} {
		if validateCallbackURL(raw) == nil {
			t.Errorf("unsafe callback URL accepted: %s", raw)
		}
	}
	if validateCallbackURL("https://chatgpt.com/receiver") != nil {
		t.Fatal("valid HTTPS callback rejected")
	}
	if _, err := decodeWebhookSecret("whsec_abc"); err == nil {
		t.Fatal("short key accepted")
	}
	if _, err := decodeWebhookSecret("invalid"); err == nil {
		t.Fatal("missing prefix accepted")
	}
	if err := verifyCallback(context.Background(), subscription{ID: "sub_test", URL: "http://localhost", Secret: "invalid"}); err == nil {
		t.Fatal("unsafe verification accepted")
	}
}
