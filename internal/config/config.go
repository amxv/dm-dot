// Package config manages the optional per-user CLI configuration. Environment
// variables override the saved values, so CI agents need no local config file.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

type Settings struct {
	APIURL string `json:"api_url"`
	Token  string `json:"token"`
}

func FilePath() (string, error) {
	root, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user config directory: %w", err)
	}
	return filepath.Join(root, "dm-dot", "config.json"), nil
}

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
	if !info.Mode().IsRegular() || (runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0) {
		return cfg, errors.New("dm-dot config must be a regular file accessible only to its owner")
	}
	file, err := os.Open(filePath)
	if err != nil {
		return cfg, err
	}
	defer file.Close()
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

func Save(cfg Settings) error {
	if strings.TrimSpace(cfg.APIURL) == "" || strings.TrimSpace(cfg.Token) == "" {
		return errors.New("both API URL and token are required")
	}
	filePath, err := FilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filePath), 0700); err != nil {
		return err
	}
	// Never follow a symlink or overwrite a credential file in place.
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
		file.Close()
		return err
	}
	if err := json.NewEncoder(file).Encode(cfg); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), filePath)
}

func Clear() error {
	filePath, err := FilePath()
	if err != nil {
		return err
	}
	err = os.Remove(filePath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

// Effective always uses env credentials if supplied, avoiding the need to
// expose a stored token to local agents or CI processes.
func Effective() (Settings, error) {
	cfg, err := Load()
	if err != nil {
		return cfg, err
	}
	if value := os.Getenv("DM_DOT_API_URL"); value != "" {
		cfg.APIURL = value
	}
	if value := os.Getenv("DM_DOT_REST_API_KEY"); value != "" {
		cfg.Token = value
	}
	return cfg, nil
}
