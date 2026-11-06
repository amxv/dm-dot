package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/amxv/dm-dot/pkg/client"
	"github.com/zalando/go-keyring"
)

type fakeKeyring struct {
	entries  map[string]string
	getCalls int
	failSet  bool
	failGet  bool
}

func (f *fakeKeyring) entry(service, user string) string { return service + "::" + user }
func (f *fakeKeyring) Get(service, user string) (string, error) {
	f.getCalls++
	if f.failGet {
		return "", errors.New("backend unavailable")
	}
	value, ok := f.entries[f.entry(service, user)]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return value, nil
}
func (f *fakeKeyring) Set(service, user, value string) error {
	if f.failSet {
		return errors.New("backend unavailable")
	}
	f.entries[f.entry(service, user)] = value
	return nil
}
func (f *fakeKeyring) Delete(service, user string) error {
	if _, ok := f.entries[f.entry(service, user)]; !ok {
		return keyring.ErrNotFound
	}
	delete(f.entries, f.entry(service, user))
	return nil
}

func setup(t *testing.T) *fakeKeyring {
	t.Helper()
	undo := SetFilePathForTesting(filepath.Join(t.TempDir(), "dm-dot", "config.json"))
	t.Cleanup(undo)
	t.Setenv("DM_DOT_API_URL", "")
	t.Setenv("DM_DOT_REST_API_KEY", "")
	store := &fakeKeyring{entries: map[string]string{}}
	previous := nativeSecrets
	nativeSecrets = store
	t.Cleanup(func() { nativeSecrets = previous })
	return store
}

func TestConfigureAndClearNativeCredentials(t *testing.T) {
	store := setup(t)
	if cfg, err := Load(); err != nil || cfg.Token != "" {
		t.Fatalf("initial load: %#v, %v", cfg, err)
	}
	if err := Configure("https://EXAMPLE.com:443/v1/messages", "secret-token"); err != nil {
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
	content, err := os.ReadFile(filePath)
	if err != nil || strings.Contains(string(content), "token") || strings.Contains(string(content), "secret-token") {
		t.Fatalf("credential persisted in JSON: %v", err)
	}
	if store.entries[store.entry(credentialService, "https://example.com")] != "secret-token" {
		t.Fatal("credential not stored under canonical origin")
	}
	cfg, err := Effective("https://example.com")
	if err != nil || cfg.Token != "secret-token" {
		t.Fatalf("effective credential: %#v, %v", cfg, err)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Fatalf("config still exists: %v", err)
	}
	if len(store.entries) != 0 {
		t.Fatalf("native credential not removed: %#v", store.entries)
	}
	if err := Clear(); err != nil {
		t.Fatal(err)
	}
}

func TestOriginOverridesFailClosedBeforeCredentialAccess(t *testing.T) {
	store := setup(t)
	if err := Configure("https://dot.example.com", "saved-key"); err != nil {
		t.Fatal(err)
	}
	for _, cliURL := range []string{
		"https://evil.example",
		"https://dot.example.com.evil.example",
		"https://dot.example.com:8443",
		"https://evil.example/v1/messages",
	} {
		before := store.getCalls
		if _, err := Effective(cliURL); !errors.Is(err, ErrDestinationMismatch) {
			t.Errorf("CLI override %q unexpectedly allowed: %v", cliURL, err)
		}
		if store.getCalls != before {
			t.Errorf("credential looked up before URL rejection: %q", cliURL)
		}
	}
	for _, withEnvSecret := range []bool{false, true} {
		t.Setenv("DM_DOT_API_URL", "https://evil.example")
		if withEnvSecret {
			t.Setenv("DM_DOT_REST_API_KEY", "environment-key")
		}
		before := store.getCalls
		if _, err := Effective(""); !errors.Is(err, ErrDestinationMismatch) {
			t.Errorf("environment override unexpectedly allowed: %v", err)
		}
		if store.getCalls != before {
			t.Error("native credential looked up before environment rejection")
		}
	}
	t.Setenv("DM_DOT_API_URL", "https://DOT.example.com:443/v1/messages")
	t.Setenv("DM_DOT_REST_API_KEY", "")
	if cfg, err := Effective(""); err != nil || cfg.Token != "saved-key" {
		t.Fatalf("same-origin env override: %#v, %v", cfg, err)
	}
	t.Setenv("DM_DOT_REST_API_KEY", "new-explicit-key")
	if cfg, err := Effective(""); err != nil || cfg.Token != "new-explicit-key" {
		t.Fatalf("same-origin token override: %#v, %v", cfg, err)
	}
	if _, err := client.Origin("https://dot.example.com:443"); err != nil {
		t.Fatal(err)
	}
}

func TestEnvironmentOnlyRequiresExplicitURLAndToken(t *testing.T) {
	setup(t)
	if _, err := Effective("https://evil.example"); !errors.Is(err, ErrDestinationMismatch) {
		t.Fatalf("CLI --url alone must not authorize credential destination: %v", err)
	}
	t.Setenv("DM_DOT_REST_API_KEY", "injected-key")
	if _, err := Effective(""); !errors.Is(err, client.ErrMissingURL) {
		t.Fatalf("key without env URL must fail: %v", err)
	}
	t.Setenv("DM_DOT_API_URL", "https://agent.example")
	if cfg, err := Effective(""); err != nil || cfg.Token != "injected-key" || cfg.APIURL != "https://agent.example" {
		t.Fatalf("environment-only credentials: %#v %v", cfg, err)
	}
	if _, err := Effective("https://other.example"); !errors.Is(err, ErrDestinationMismatch) {
		t.Fatalf("CLI override must not redirect env key: %v", err)
	}
}

func TestLegacyMigrationVerifiedBeforePlaintextRemoval(t *testing.T) {
	store := setup(t)
	filePath, err := FilePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		t.Fatal(err)
	}
	old := `{"api_url":"https://DM-dot.example","token":"legacy-secret"}` + "\n"
	if err := os.WriteFile(filePath, []byte(old), 0600); err != nil {
		t.Fatal(err)
	}
	store.failSet = true
	if _, err := Effective(""); !errors.Is(err, ErrMigrationFailed) {
		t.Fatalf("expected migration failure with no plaintext deletion, got %v", err)
	}
	contents, _ := os.ReadFile(filePath)
	if string(contents) != old {
		t.Fatal("plaintext removed despite keyring failure")
	}
	store.failSet = false
	store.failGet = true
	if _, err := Effective(""); !errors.Is(err, ErrMigrationFailed) {
		t.Fatalf("readback verification failure: %v", err)
	}
	contents, _ = os.ReadFile(filePath)
	if string(contents) != old {
		t.Fatal("plaintext removed before readable native key was confirmed")
	}
	store.failGet = false
	cfg, err := Effective("")
	if err != nil || cfg.Token != "legacy-secret" {
		t.Fatalf("migration failed: %#v, %v", cfg, err)
	}
	contents, _ = os.ReadFile(filePath)
	if strings.Contains(string(contents), "token") || strings.Contains(string(contents), "legacy-secret") {
		t.Fatal("legacy token still in config after successful migration")
	}
	if store.entries[store.entry(credentialService, "https://dm-dot.example")] != "legacy-secret" {
		t.Fatal("migrated secret not in native store")
	}
	if cfg, err := Effective(""); err != nil || cfg.Token != "legacy-secret" {
		t.Fatalf("native repeat read failed: %#v, %v", cfg, err)
	}
}

func TestChangingDestinationReplacesOriginCredential(t *testing.T) {
	store := setup(t)
	if err := Configure("https://old.example", "old-secret"); err != nil {
		t.Fatal(err)
	}
	if err := Configure("https://new.example", "new-secret"); err != nil {
		t.Fatal(err)
	}
	if len(store.entries) != 1 || store.entries[store.entry(credentialService, "https://new.example")] != "new-secret" {
		t.Fatalf("stale credential remains after reconfigure: %#v", store.entries)
	}
}

func TestRejectInsecureConfigAndUnavailableSecretStore(t *testing.T) {
	store := setup(t)
	store.failSet = true
	if err := Configure("https://valid.example", "secret"); !errors.Is(err, ErrSecretUnavailable) {
		t.Fatalf("unavailable store should fail closed: %v", err)
	}
	filePath, _ := FilePath()
	if _, err := os.Stat(filePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("should not create plaintext fallback config")
	}
	if runtime.GOOS == "windows" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filePath, []byte(`{"api_url":"https://example.com","token":"secret"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil {
		t.Fatal("expected insecure file permission rejection")
	}
}
