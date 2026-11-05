package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSaveLoadClearAndEnvironment(t *testing.T) {
	root := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("AppData", root)
	} else {
		t.Setenv("XDG_CONFIG_HOME", root)
	}
	t.Setenv("DM_DOT_API_URL", "")
	t.Setenv("DM_DOT_REST_API_KEY", "")
	if cfg, err := Load(); err != nil || cfg.Token != "" {
		t.Fatalf("unexpected initial config: %#v, %v", cfg, err)
	}
	initial := Settings{APIURL: "https://example.com", Token: "secret"}
	if err := Save(initial); err != nil {
		t.Fatal(err)
	}
	filePath, err := FilePath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0600 {
		t.Fatalf("file mode = %v", info.Mode().Perm())
	}
	if cfg, err := Effective(); err != nil || cfg != initial {
		t.Fatalf("unexpected config: %#v, %v", cfg, err)
	}
	t.Setenv("DM_DOT_API_URL", "https://override.example")
	t.Setenv("DM_DOT_REST_API_KEY", "env-secret")
	if cfg, err := Effective(); err != nil || cfg.APIURL != "https://override.example" || cfg.Token != "env-secret" {
		t.Fatalf("environment overrides: %#v, %v", cfg, err)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("config still exists: %v", err)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
}

func TestRejectInsecureConfig(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows uses inherited ACLs instead of POSIX modes")
	}
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	filePath, err := FilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(`{"api_url":"https://example.com","token":"secret"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected insecure mode rejection")
	}
}
