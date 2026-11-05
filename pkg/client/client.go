// Package client sends messages to a deployed dm-dot instance over its REST
// API. Local callers do not need Redis, MCP, or webhook-signing credentials.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/amxv/dm-dot/pkg/message"
)

var (
	ErrMissingURL   = errors.New("set DM_DOT_API_URL or pass --url")
	ErrMissingToken = errors.New("set DM_DOT_REST_API_KEY to your REST bearer token")
	ErrInvalidURL   = errors.New("API URL must be an HTTPS origin or its /v1/messages endpoint, without credentials, query, or fragment")
	ErrJSONTooLarge = errors.New("encoded message exceeds the REST API's 32 KiB request limit")
)

const maxResponseBytes = 16 * 1024

type Client struct {
	APIURL     string
	Token      string
	HTTPClient *http.Client
}

// RemoteError exposes a sanitized HTTP status and the documented REST error
// code. It never includes the bearer token or an arbitrary response body.
type RemoteError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *RemoteError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("dm-dot returned HTTP %d", e.StatusCode)
	}
	if e.Message == "" {
		return fmt.Sprintf("dm-dot returned HTTP %d (%s)", e.StatusCode, e.Code)
	}
	return fmt.Sprintf("dm-dot returned HTTP %d (%s): %s", e.StatusCode, e.Code, e.Message)
}

// Send returns when ChatGPT acknowledges the webhook, NOT when the dot has
// finished processing the message. It intentionally does not retry requests:
// a transport failure may occur after the server has already delivered one.
func (c Client) Send(ctx context.Context, input string) (message.Accepted, error) {
	var zero message.Accepted
	text, err := message.Normalize(input)
	if err != nil {
		return zero, err
	}
	endpoint, err := endpointURL(c.APIURL)
	if err != nil {
		return zero, err
	}
	if strings.TrimSpace(c.Token) == "" {
		return zero, ErrMissingToken
	}

	data, err := json.Marshal(message.Request{Message: text})
	if err != nil {
		return zero, err
	}
	if len(data) > message.MaxRESTBodyBytes {
		return zero, ErrJSONTooLarge
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return zero, fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Authorization", "Bearer "+c.Token)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 45 * time.Second}
	}
	// A redirect is an error, never a reason to forward the bearer token.
	noRedirect := *httpClient
	noRedirect.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}
	response, err := noRedirect.Do(request)
	if err != nil {
		return zero, fmt.Errorf("send message: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponseBytes+1))
	if err != nil || len(body) > maxResponseBytes {
		return zero, errors.New("invalid or oversized response from dm-dot")
	}
	if response.StatusCode != http.StatusAccepted {
		remote := &RemoteError{StatusCode: response.StatusCode}
		var payload message.APIError
		if json.Unmarshal(body, &payload) == nil {
			remote.Code = payload.Error.Code
			remote.Message = payload.Error.Message
		}
		return zero, remote
	}
	var accepted message.Accepted
	if json.Unmarshal(body, &accepted) != nil || accepted.Status != "accepted" || accepted.EventID == "" {
		return zero, errors.New("dm-dot returned an invalid acceptance response")
	}
	return accepted, nil
}

func endpointURL(raw string) (string, error) {
	if strings.TrimSpace(raw) == "" {
		return "", ErrMissingURL
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return "", ErrInvalidURL
	}
	switch u.EscapedPath() {
	case "", "/", "/v1/messages":
		u.Path = "/v1/messages"
		u.RawPath = ""
	default:
		return "", ErrInvalidURL
	}
	return u.String(), nil
}
