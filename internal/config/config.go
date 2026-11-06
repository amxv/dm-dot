// Package config binds each CLI credential to a single REST API origin and
// stores secrets in the operating system's credential store. The JSON file
// contains only the deployment URL. Version 0.1 plaintext files are migrated
// atomically before they can be used.
package config

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/amxv/dm-dot/pkg/client"
	"github.com/zalando/go-keyring"
)

const credentialService = "dm-dot-cli"

var (
	ErrDestinationMismatch = errors.New("API origin differs from the configured credential; run 'dm-dot configure' to change it")
	ErrSecretUnavailable   = errors.New("could not access the native credential store; unlock it or provide DM_DOT_API_URL and DM_DOT_REST_API_KEY on a machine without a saved configuration")
	ErrMigrationFailed     = errors.New("could not migrate v0.1 plaintext token into the native credential store; original config remains unchanged")
)

// Settings is the runtime configuration. Token never appears in newly saved
// files; a v0.1 Token field is recognized only for migration.
type Settings struct {
	APIURL string `json:"api_url"`
	Token  string `json:"token,omitempty"`
}

// secretStore is mockable, but production always uses the OS credential API.
type secretStore interface {
	Get(service, user string) (string, error)
	Set(service, user, password string) error
	Delete(service, user string) error
}

type osCredentialStore struct{}

func (osCredentialStore) Get(service, user string) (string, error) {
	return keyring.Get(service, user)
}
func (osCredentialStore) Set(service, user, password string) error {
	return keyring.Set(service, user, password)
}
func (osCredentialStore) Delete(service, user string) error {
	return keyring.Delete(service, user)
}

var nativeSecrets secretStore = osCredentialStore{}

// configFilePath is injected by in-process tests, never by an environment
// variable or CLI flag. This protects a real macOS user's ~/Library config
// from test fixtures (os.UserConfigDir ignores XDG_CONFIG_HOME on macOS).
var configFilePath = defaultConfigFilePath

func defaultConfigFilePath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user config directory: %w", err)
	}
	return filepath.Join(root, "dm-dot", "config.json"), nil
}

func FilePath() (string, error) { return configFilePath() }

// SetFilePathForTesting is only for hermetic package tests. The real CLI has
// no way to override the credential-file location at runtime.
func SetFilePathForTesting(filePath string) func() {
	previous := configFilePath
	configFilePath = func() (string, error) { return filePath, nil }
	return func() { configFilePath = previous }
}

// Load reads disk configuration, including a legacy v0.1 token if present.
// Do not use Load for sending: Effective enforces origin binding and migrates
// any legacy token before returning it.
func Load() (Settings, error) {
	var cfg Settings
	filePath, err := FilePath()
	if err != nil {
		return cfg, err
	}
	info, err := os.Lstat(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if info.Size() > 8192 {
		return cfg, errors.New("dm-dot config exceeds the maximum supported size")
	}
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return cfg, errors.New("dm-dot config must be a regular file accessible only to its owner")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return cfg, err
	}
	defer file.Close()
	// Recheck permissions after opening to avoid a stale pre-open stat.
	info, err = file.Stat()
	if err != nil || !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return cfg, errors.New("dm-dot config became unreadable or insecure")
	}
	decoder := json.NewDecoder(io.LimitReader(file, 8192))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&cfg); err != nil {
		return Settings{}, fmt.Errorf("read dm-dot config: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return Settings{}, errors.New("dm-dot config contains extra JSON data")
	}
	return cfg, nil
}

// saveURL never serializes a bearer token. Atomic rename replaces legacy
// configs only after verified storage in the OS credential manager.
func saveURL(apiURL string) error {
	filePath, err := FilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		return err
	}
	if info, err := os.Lstat(filePath); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("refusing to replace non-regular config file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(filePath), ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return err
	}
	if err := json.NewEncoder(file).Encode(struct {
		APIURL string `json:"api_url"`
	}{apiURL}); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Sync(); err != nil {
		_ = file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filePath)
}

// storeAndVerify ensures a secret is readable before the plaintext source can
// be removed. Never include credential values or raw backend errors in output.
func storeAndVerify(origin, token string) error {
	if err := nativeSecrets.Set(credentialService, origin, token); err != nil {
		return ErrSecretUnavailable
	}
	value, err := nativeSecrets.Get(credentialService, origin)
	if err != nil || subtle.ConstantTimeCompare([]byte(value), []byte(token)) != 1 {
		return ErrSecretUnavailable
	}
	return nil
}

func migrateLegacy(cfg *Settings) error {
	if cfg.Token == "" {
		return nil
	}
	origin, err := client.Origin(cfg.APIURL)
	if err != nil {
		return fmt.Errorf("cannot migrate invalid legacy API URL: %w", err)
	}
	if err := storeAndVerify(origin, cfg.Token); err != nil {
		return ErrMigrationFailed
	}
	if err := saveURL(cfg.APIURL); err != nil {
		return fmt.Errorf("cannot remove plaintext token after secure storage: %w", err)
	}
	cfg.Token = ""
	return nil
}

// Configure saves an origin-scoped token in Keychain, Credential Manager or
// Secret Service, and only stores the API URL on disk. Never silently fall back
// to a plaintext file when an OS credential store is unavailable.
func Configure(apiURL, token string) error {
	origin, err := client.Origin(apiURL)
	if err != nil {
		return err
	}
	if token == "" || strings.TrimSpace(token) != token || strings.ContainsAny(token, " \n\r\t") {
		return errors.New("REST key must be a nonempty bearer token")
	}
	previous, err := Load()
	if err != nil {
		return err
	}
	oldValue, oldErr := nativeSecrets.Get(credentialService, origin)
	if oldErr != nil && !errors.Is(oldErr, keyring.ErrNotFound) {
		return ErrSecretUnavailable
	}
	if err := storeAndVerify(origin, token); err != nil {
		return err
	}
	if err := saveURL(apiURL); err != nil {
		// Do not lose a working native credential if saving the config fails.
		if oldErr == nil {
			_ = nativeSecrets.Set(credentialService, origin, oldValue)
		} else {
			_ = nativeSecrets.Delete(credentialService, origin)
		}
		return err
	}
	if previous.APIURL != "" {
		if prevOrigin, err := client.Origin(previous.APIURL); err == nil && prevOrigin != origin {
			// Best-effort cleanup of a previous origin; it is no longer usable.
			_ = nativeSecrets.Delete(credentialService, prevOrigin)
		}
	}
	return nil
}

// Effective determines the only safe destination before retrieving a secret.
// Environment or CLI URL overrides are allowed only on the same pinned origin.
// For stateless agents with no saved config, BOTH environment variables must
// be provided to explicitly bind an environment credential to its destination.
func Effective(cliURL string) (Settings, error) {
	cfg, err := Load()
	if err != nil {
		return Settings{}, err
	}
	if err := migrateLegacy(&cfg); err != nil {
		return Settings{}, err
	}
	fileOrigin := ""
	if cfg.APIURL != "" {
		fileOrigin, err = client.Origin(cfg.APIURL)
		if err != nil {
			return Settings{}, err
		}
	}
	envURL := os.Getenv("DM_DOT_API_URL")
	envToken := os.Getenv("DM_DOT_REST_API_KEY")
	selectedOrigin := fileOrigin
	if envURL != "" {
		origin, err := client.Origin(envURL)
		if err != nil {
			return Settings{}, err
		}
		if selectedOrigin != "" && origin != selectedOrigin {
			return Settings{}, ErrDestinationMismatch
		}
		selectedOrigin = origin
		cfg.APIURL = envURL
	}
	if cliURL != "" {
		origin, err := client.Origin(cliURL)
		if err != nil {
			return Settings{}, err
		}
		if selectedOrigin == "" || origin != selectedOrigin {
			return Settings{}, ErrDestinationMismatch
		}
		cfg.APIURL = cliURL
	}
	if selectedOrigin == "" {
		return Settings{}, client.ErrMissingURL
	}
	if envToken != "" {
		cfg.Token = envToken
		return cfg, nil
	}
	if fileOrigin == "" {
		return Settings{}, client.ErrMissingToken
	}
	secret, err := nativeSecrets.Get(credentialService, fileOrigin)
	if err != nil {
		if errors.Is(err, keyring.ErrNotFound) {
			return Settings{}, errors.New("no native credential for configured API origin; run 'dm-dot configure'")
		}
		return Settings{}, ErrSecretUnavailable
	}
	cfg.Token = secret
	return cfg, nil
}

// Clear removes the native token tied to the configured origin and its local
// URL configuration. A legacy plaintext config can be cleared without first
// migrating it, even when the OS keyring is locked or unavailable.
func Clear() error {
	cfg, err := Load()
	if err != nil {
		return err
	}
	if cfg.APIURL != "" {
		origin, err := client.Origin(cfg.APIURL)
		if err != nil {
			return err
		}
		if err := nativeSecrets.Delete(credentialService, origin); err != nil && !errors.Is(err, keyring.ErrNotFound) {
			return ErrSecretUnavailable
		}
	}
	filePath, err := FilePath()
	if err != nil {
		return err
	}
	if err := os.Remove(filePath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
