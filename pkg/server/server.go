package server

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var transport = mcp.NewStreamableHTTPHandler(
	func(*http.Request) *mcp.Server { return makeServer() },
	&mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true},
)

// Handler serves the MCP tool and the draft MCP Events methods over the same
// authenticated endpoint. Events currently have to be handled outside the Go
// SDK because its server capability schema has no top-level events field.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Robots-Tag", "noindex")

		secret := strings.TrimSpace(os.Getenv("MCP_API_KEY"))
		if secret == "" {
			http.Error(w, "MCP endpoint not configured", http.StatusServiceUnavailable)
			return
		}
		keys := r.URL.Query()["key"]
		if len(keys) != 1 || !validKey(secret, keys[0]) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			transport.ServeHTTP(w, r)
			return
		}

		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20+1))
		if err != nil || len(body) > 1<<20 {
			http.Error(w, "Invalid request size", http.StatusRequestEntityTooLarge)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Method string          `json:"method"`
			ID     json.RawMessage `json:"id"`
			Params json.RawMessage `json:"params"`
		}
		if json.Unmarshal(body, &request) != nil {
			transport.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(request.Method, "events/") {
			handleEventRPC(w, r, request.Method, request.ID, request.Params)
			return
		}
		if request.Method == "initialize" || request.Method == "server/discover" {
			// ChatGPT discovers events from initialize on legacy MCP and from
			// server/discover on MCP 2.0 (2026-07-28). The upstream Go SDK does
			// not expose the draft events capability in its typed schema, so
			// advertise it in both handshake responses. Keep all other fields
			// and version negotiation owned by the SDK.
			recorder := httptest.NewRecorder()
			transport.ServeHTTP(recorder, r)
			result := recorder.Result()
			defer result.Body.Close()
			out := recorder.Body.Bytes()
			if result.StatusCode == http.StatusOK {
				var envelope map[string]json.RawMessage
				if json.Unmarshal(out, &envelope) == nil && len(envelope["result"]) != 0 {
					var payload map[string]json.RawMessage
					if json.Unmarshal(envelope["result"], &payload) == nil {
						var capabilities map[string]json.RawMessage
						if json.Unmarshal(payload["capabilities"], &capabilities) == nil {
							capabilities["events"] = json.RawMessage(`{}`)
							payload["capabilities"], _ = json.Marshal(capabilities)
							envelope["result"], _ = json.Marshal(payload)
							out, _ = json.Marshal(envelope)
						}
					}
				}
			}
			for key, values := range result.Header {
				for _, value := range values {
					w.Header().Add(key, value)
				}
			}
			w.Header().Del("Content-Length")
			w.WriteHeader(result.StatusCode)
			_, _ = w.Write(out)
			return
		}
		transport.ServeHTTP(w, r)
	})
}

func validKey(expected, supplied string) bool {
	if supplied == "" {
		return false
	}
	want := sha256.Sum256([]byte(expected))
	got := sha256.Sum256([]byte(supplied))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}
