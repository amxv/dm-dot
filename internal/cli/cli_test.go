package cli

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func prepareConfig(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", t.TempDir())
	} else {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	}
	t.Setenv("DM_DOT_REST_API_KEY", "")
	t.Setenv("DM_DOT_API_URL", "")
}

func run(t *testing.T, args []string, stdin string, httpClient *http.Client) (string, string, error) {
	t.Helper()
	var out, stderr bytes.Buffer
	err := Run(context.Background(), args, strings.NewReader(stdin), &out, &stderr, httpClient)
	return out.String(), stderr.String(), err
}

func TestConfigureSendAndClear(t *testing.T) {
	prepareConfig(t)
	var received string
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Error("missing REST authorization")
		}
		data, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		received = string(data)
		w.WriteHeader(202)
		_, _ = io.WriteString(w, `{"status":"accepted","event_id":"evt_test"}`)
	}))
	defer server.Close()
	out, _, err := run(t, []string{"configure", "--url", server.URL, "--token-stdin"}, "secret-token\n", nil)
	if err != nil || strings.Contains(out, "secret-token") {
		t.Fatalf("configure: %q %v", out, err)
	}
	out, _, err = run(t, []string{"config", "show"}, "", nil)
	if err != nil || !strings.Contains(out, server.URL) || !strings.Contains(out, "configured") || strings.Contains(out, "secret-token") {
		t.Fatalf("config show: %q %v", out, err)
	}
	out, _, err = run(t, []string{"send", "--json", "hello", "from", "cli"}, "", server.Client())
	if err != nil || strings.TrimSpace(out) != `{"status":"accepted","event_id":"evt_test"}` || received != `{"message":"hello from cli"}` {
		t.Fatalf("send JSON: out=%q received=%q err=%v", out, received, err)
	}
	out, _, err = run(t, []string{"send"}, "hello from stdin\n", server.Client())
	if err != nil || !strings.Contains(out, "evt_test") || received != `{"message":"hello from stdin"}` {
		t.Fatalf("send stdin: out=%q received=%q err=%v", out, received, err)
	}
	filePath := filepath.Join(t.TempDir(), "message.txt")
	if err := os.WriteFile(filePath, []byte("message from file"), 0600); err != nil {
		t.Fatal(err)
	}
	out, _, err = run(t, []string{"send", "--file", filePath}, "", server.Client())
	if err != nil || received != `{"message":"message from file"}` {
		t.Fatalf("send file: out=%q received=%q err=%v", out, received, err)
	}
	_, _, err = run(t, []string{"config", "clear"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = run(t, []string{"send", "hello"}, "", server.Client())
	if err == nil {
		t.Fatal("expected missing URL after clearing config")
	}
}

func TestConfigureRejectsUnsafeInputs(t *testing.T) {
	prepareConfig(t)
	for _, tc := range []struct {
		args  []string
		stdin string
	}{
		{[]string{"configure", "--url", "http://example.com", "--token-stdin"}, "secret"},
		{[]string{"configure", "--url", "https://example.com?key=secret", "--token-stdin"}, "secret"},
		{[]string{"configure", "--url", "https://example.com", "--token-stdin"}, "a b"},
		{[]string{"configure", "--url", "https://example.com"}, "secret"},
		{[]string{"send", "--file", "file", "also-a-message"}, ""},
	} {
		if _, _, err := run(t, tc.args, tc.stdin, nil); err == nil {
			t.Errorf("expected error for %#v", tc.args)
		}
	}
}

func TestCLIHelpAndVersion(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{nil, "dm-dot -"},
		{[]string{"--help"}, "Usage:"},
		{[]string{"send", "--help"}, "Usage: dm-dot send"},
		{[]string{"--version"}, "dm-dot v"},
	} {
		out, stderr, err := run(t, tc.args, "", nil)
		if err != nil || !strings.Contains(out+stderr, tc.want) {
			t.Errorf("args=%v, stdout=%q stderr=%q error=%v", tc.args, out, stderr, err)
		}
	}
}
