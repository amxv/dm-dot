// Package message contains the shared message contract and validation used by
// the MCP, REST, and CLI entry points.
package message

import (
	"errors"
	"strings"
)

const (
	MaxBytes         = 16 * 1024
	MaxRESTBodyBytes = 32 * 1024
)

var ErrInvalidMessage = errors.New("message must contain between 1 and 16384 UTF-8 bytes")

// Normalize applies the exact same trimming and byte limit for every sender.
func Normalize(input string) (string, error) {
	value := strings.TrimSpace(input)
	if len(value) == 0 || len(value) > MaxBytes {
		return "", ErrInvalidMessage
	}
	return value, nil
}

type Request struct {
	Message string `json:"message"`
}

// Accepted confirms webhook acknowledgement, not completion by the dot.
type Accepted struct {
	Status  string `json:"status"`
	EventID string `json:"event_id"`
}

type APIError struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}
