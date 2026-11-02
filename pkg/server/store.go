package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const subscriptionKey = "dm-dot:subscription:v1"

// Subscription is intentionally singular. The signing secret is held only
// in encrypted-at-rest deployment secrets and the private Redis database.
type subscription struct {
	ID        string    `json:"id"`
	URL       string    `json:"url"`
	Secret    string    `json:"secret"`
	ExpiresAt time.Time `json:"expires_at"`
}

type subscriptionStore struct{}

var newRedisClient = func() *http.Client { return &http.Client{Timeout: 8 * time.Second} }

func (subscriptionStore) command(ctx context.Context, args ...any) (json.RawMessage, error) {
	endpoint := strings.TrimRight(strings.TrimSpace(os.Getenv("UPSTASH_REDIS_REST_URL")), "/")
	token := os.Getenv("UPSTASH_REDIS_REST_TOKEN")
	if endpoint == "" || token == "" {
		return nil, errors.New("persistent subscription storage is not configured")
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return nil, errors.New("invalid Redis REST URL")
	}
	body, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	client := newRedisClient()
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Redis unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Redis returned HTTP %d", resp.StatusCode)
	}
	var result struct {
		Result json.RawMessage `json:"result"`
		Error  string          `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid Redis response: %w", err)
	}
	if result.Error != "" {
		return nil, errors.New("Redis command failed")
	}
	return result.Result, nil
}

func (s subscriptionStore) get(ctx context.Context) (*subscription, error) {
	raw, err := s.command(ctx, "GET", subscriptionKey)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var serialized string
	if err := json.Unmarshal(raw, &serialized); err != nil {
		return nil, errors.New("invalid stored subscription")
	}
	var sub subscription
	if err := json.Unmarshal([]byte(serialized), &sub); err != nil || sub.ID == "" {
		return nil, errors.New("corrupt stored subscription")
	}
	if !time.Now().Before(sub.ExpiresAt) {
		return nil, nil
	}
	return &sub, nil
}

func (s subscriptionStore) set(ctx context.Context, sub subscription) error {
	data, err := json.Marshal(sub)
	if err != nil {
		return err
	}
	ttl := int(time.Until(sub.ExpiresAt).Seconds())
	if ttl < 1 {
		return errors.New("subscription already expired")
	}
	_, err = s.command(ctx, "SET", subscriptionKey, string(data), "EX", ttl)
	return err
}

func (s subscriptionStore) delete(ctx context.Context) error {
	_, err := s.command(ctx, "DEL", subscriptionKey)
	return err
}
