package message

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	for _, tc := range []struct {
		name, input, want string
		invalid           bool
	}{
		{name: "trim", input: " \n hello \t", want: "hello"},
		{name: "exact limit", input: strings.Repeat("a", MaxBytes), want: strings.Repeat("a", MaxBytes)},
		{name: "UTF-8 byte limit", input: strings.Repeat("💬", MaxBytes/4+1), invalid: true},
		{name: "empty", input: "\t \n", invalid: true},
		{name: "too long", input: strings.Repeat("a", MaxBytes+1), invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Normalize(tc.input)
			if tc.invalid {
				if !errors.Is(err, ErrInvalidMessage) {
					t.Fatalf("got %q, %v, want ErrInvalidMessage", got, err)
				}
			} else if err != nil || got != tc.want {
				t.Fatalf("got %q, %v, want %q", got, err, tc.want)
			}
		})
	}
}
