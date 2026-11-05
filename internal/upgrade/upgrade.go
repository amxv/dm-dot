// Package upgrade provides an explicit, checksum-verified native upgrade path
// through the same website-hosted installers used for the initial install.
package upgrade

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/amxv/dm-dot/internal/buildinfo"
)

const (
	latestPage    = "https://github.com/amxv/dm-dot/releases/latest"
	installerBase = "https://dm-dot.ashray.xyz"
	tagPrefix     = "/amxv/dm-dot/releases/tag/v"
)

func Run(ctx context.Context, args []string, out io.Writer) error {
	check := len(args) == 1 && args[0] == "--check"
	if len(args) > 1 || (len(args) == 1 && !check) {
		return errors.New("usage: dm-dot upgrade [--check]")
	}
	latest, err := latestVersion(ctx)
	if err != nil {
		return fmt.Errorf("check GitHub releases: %w", err)
	}
	current := buildinfo.Current()
	if check {
		_, _ = fmt.Fprintf(out, "Installed: %s\nLatest: %s\n", current, latest)
	}
	if current == "dev" {
		if check {
			_, _ = io.WriteString(out, "Development build; install a release to enable upgrades.\n")
			return nil
		}
		return errors.New("cannot upgrade a development build; install a published release")
	}
	newer, err := newerVersion(latest, current)
	if err != nil {
		return err
	}
	if !newer {
		if !check {
			_, _ = io.WriteString(out, "Already up to date.\n")
		}
		return nil
	}
	if check {
		_, _ = io.WriteString(out, "Update available.\n")
		return nil
	}
	return install(ctx, latest, out)
}

func parseVersion(version string) ([3]uint64, error) {
	var numbers [3]uint64
	values := strings.Split(strings.TrimPrefix(version, "v"), ".")
	if len(values) != 3 {
		return numbers, fmt.Errorf("invalid stable version %q", version)
	}
	for i, part := range values {
		if part == "" {
			return numbers, fmt.Errorf("invalid stable version %q", version)
		}
		for _, digit := range part {
			if digit < '0' || digit > '9' {
				return numbers, fmt.Errorf("invalid stable version %q", version)
			}
		}
		value, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return numbers, err
		}
		numbers[i] = value
	}
	return numbers, nil
}

func newerVersion(latest, current string) (bool, error) {
	next, err := parseVersion(latest)
	if err != nil {
		return false, err
	}
	installed, err := parseVersion(current)
	if err != nil {
		return false, err
	}
	for i := range next {
		if next[i] != installed[i] {
			return next[i] > installed[i], nil
		}
	}
	return false, nil
}

func tagFromLocation(location string) (string, error) {
	page, _ := url.Parse(latestPage)
	redirect, err := url.Parse(location)
	if err != nil {
		return "", err
	}
	resolved := page.ResolveReference(redirect)
	if resolved.Scheme != "https" || resolved.Host != "github.com" || resolved.User != nil || resolved.RawQuery != "" || resolved.Fragment != "" || !strings.HasPrefix(resolved.Path, tagPrefix) {
		return "", errors.New("untrusted GitHub latest release redirect")
	}
	version := strings.TrimPrefix(resolved.Path, tagPrefix)
	if _, err := parseVersion(version); err != nil {
		return "", err
	}
	return version, nil
}

func latestVersion(ctx context.Context) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodHead, latestPage, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{
		Timeout:       20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	switch response.StatusCode {
	case http.StatusMovedPermanently, http.StatusFound, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		return tagFromLocation(response.Header.Get("Location"))
	default:
		return "", fmt.Errorf("GitHub latest release returned HTTP %d", response.StatusCode)
	}
}

func install(ctx context.Context, version string, out io.Writer) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	executable, err = filepath.EvalSymlinks(executable)
	if err != nil {
		return err
	}
	name := "install.sh"
	if runtime.GOOS == "windows" {
		name = "install.ps1"
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, installerBase+"/"+name, nil)
	if err != nil {
		return err
	}
	httpClient := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || req.URL.Hostname() != "dm-dot.ashray.xyz" {
			return errors.New("installer redirected outside the documentation website")
		}
		if len(via) >= 4 {
			return errors.New("too many installer redirects")
		}
		return nil
	}}
	response, err := httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("download website installer: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("website installer returned HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 128*1024+1))
	if err != nil || len(data) > 128*1024 {
		return errors.New("installer download invalid or too large")
	}
	file, err := os.CreateTemp("", "dm-dot-upgrade-*"+filepath.Ext(name))
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return err
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		return err
	}
	if runtime.GOOS == "windows" {
		command := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", file.Name(), "-Upgrade", "-Target", executable, "-WaitForPid", strconv.Itoa(os.Getpid()))
		command.Env = append(os.Environ(), "CLI_VERSION="+version)
		command.Stdout, command.Stderr = out, os.Stderr
		if err := command.Start(); err != nil {
			_ = os.Remove(file.Name())
			return err
		}
		_ = command.Process.Release()
		_, _ = io.WriteString(out, "Upgrade started; Windows replacement completes after this command exits.\n")
		return nil
	}
	defer os.Remove(file.Name())
	command := exec.CommandContext(ctx, "sh", file.Name(), "upgrade")
	command.Env = append(os.Environ(), "CLI_UPGRADE_TARGET="+executable, "CLI_VERSION="+version)
	command.Stdout, command.Stderr = out, os.Stderr
	return command.Run()
}
