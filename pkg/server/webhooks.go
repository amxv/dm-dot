package server

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Injectable only by package tests. The production transport re-resolves DNS
// for each connection and refuses non-public addresses, redirects and proxies.
var newCallbackClient = func() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			Proxy:               nil,
			DisableKeepAlives:   true,
			TLSHandshakeTimeout: 5 * time.Second,
			DialContext:         dialPublicAddress,
		},
	}
}

func validateCallbackURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return errors.New("callback must use HTTPS without credentials or fragments")
	}
	if port := u.Port(); port != "" && port != "443" {
		return errors.New("callback must use HTTPS port 443")
	}
	if strings.EqualFold(u.Hostname(), "localhost") || strings.HasSuffix(strings.ToLower(u.Hostname()), ".localhost") {
		return errors.New("local callback is forbidden")
	}
	if ip, err := netip.ParseAddr(u.Hostname()); err == nil && !isPublicIP(ip) {
		return errors.New("private callback IP is forbidden")
	}
	return nil
}

func isPublicIP(ip netip.Addr) bool {
	ip = ip.Unmap()
	if !ip.IsValid() || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	// Also reject shared carrier-grade NAT and benchmarking networks. They are
	// not classified as private by netip but must not be webhook targets.
	for _, cidr := range []string{"100.64.0.0/10", "198.18.0.0/15", "192.0.0.0/24", "192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24", "240.0.0.0/4"} {
		if netip.MustParsePrefix(cidr).Contains(ip) {
			return false
		}
	}
	return true
}

func dialPublicAddress(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil || port != "443" {
		return nil, errors.New("invalid callback connection")
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil || len(addresses) == 0 {
		return nil, errors.New("callback DNS unavailable")
	}
	for _, address := range addresses {
		ip, ok := netip.AddrFromSlice(address.IP)
		if !ok || !isPublicIP(ip) {
			return nil, errors.New("callback DNS contains a non-public address")
		}
	}
	return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
}

func decodeWebhookSecret(encoded string) ([]byte, error) {
	if !strings.HasPrefix(encoded, "whsec_") {
		return nil, errors.New("invalid secret prefix")
	}
	value := strings.TrimPrefix(encoded, "whsec_")
	key, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		key, err = base64.RawStdEncoding.DecodeString(value)
	}
	if err != nil || len(key) < 24 || len(key) > 64 {
		return nil, errors.New("invalid signing key size")
	}
	return key, nil
}

func signedWebhook(secret, eventID string, timestamp int64, body []byte) (string, error) {
	key, err := decodeWebhookSecret(secret)
	if err != nil {
		return "", err
	}
	signer := hmac.New(sha256.New, key)
	_, _ = signer.Write([]byte(eventID + "." + strconv.FormatInt(timestamp, 10) + "."))
	_, _ = signer.Write(body)
	return "v1," + base64.StdEncoding.EncodeToString(signer.Sum(nil)), nil
}

func randomID(prefix string) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(b), nil
}

func postSignedWebhook(ctx context.Context, sub subscription, eventID string, body []byte) (int, []byte, error) {
	if len(body) > 256*1024 {
		return 0, nil, errors.New("webhook event too large")
	}
	if err := validateCallbackURL(sub.URL); err != nil {
		return 0, nil, err
	}
	timestamp := time.Now().Unix()
	signature, err := signedWebhook(sub.Secret, eventID, timestamp, body)
	if err != nil {
		return 0, nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.URL, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("webhook-id", eventID)
	req.Header.Set("webhook-timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("webhook-signature", signature)
	req.Header.Set("X-MCP-Subscription-Id", sub.ID)
	resp, err := newCallbackClient().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil || len(data) > 4096 {
		return 0, nil, errors.New("callback response exceeded limit")
	}
	return resp.StatusCode, data, nil
}

func verifyCallback(ctx context.Context, sub subscription) error {
	challenge, err := randomID("challenge_")
	if err != nil {
		return err
	}
	verificationID, err := randomID("msg_verification_")
	if err != nil {
		return err
	}
	body, _ := json.Marshal(map[string]any{"type": "verification", "challenge": challenge})
	status, response, err := postSignedWebhook(ctx, sub, verificationID, body)
	if err != nil || status < 200 || status >= 300 {
		return errors.New("callback verification rejected")
	}
	var reply struct {
		Challenge string `json:"challenge"`
	}
	if json.Unmarshal(response, &reply) != nil || subtle.ConstantTimeCompare([]byte(reply.Challenge), []byte(challenge)) != 1 {
		return errors.New("callback challenge mismatch")
	}
	return nil
}

func deliverMessage(ctx context.Context, sub subscription, message string) (string, error) {
	eventID, err := randomID("evt_")
	if err != nil {
		return "", err
	}
	body, _ := json.Marshal(map[string]any{
		"eventId": eventID, "name": eventName,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"data":      map[string]any{"message": message}, "cursor": nil,
	})
	for attempt := 0; attempt < 3; attempt++ {
		status, _, err := postSignedWebhook(ctx, sub, eventID, body)
		if err == nil && status >= 200 && status < 300 {
			return eventID, nil
		}
		if err == nil && status != 429 && status < 500 {
			return "", fmt.Errorf("callback returned HTTP %d", status)
		}
		if attempt == 2 {
			if err != nil {
				return "", fmt.Errorf("callback unavailable: %w", err)
			}
			return "", fmt.Errorf("callback returned HTTP %d", status)
		}
		delay := time.Duration(200*(1<<attempt)) * time.Millisecond
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(delay):
		}
	}
	return "", errors.New("delivery exhausted")
}
