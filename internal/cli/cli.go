// Package cli implements the local dm-dot executable. It shares the REST
// payload and validation contract with the MCP server while using HTTPS as the
// only bridge from a local agent to the deployed subscription delivery core.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/amxv/dm-dot/internal/buildinfo"
	"github.com/amxv/dm-dot/internal/config"
	"github.com/amxv/dm-dot/internal/upgrade"
	"github.com/amxv/dm-dot/pkg/client"
	"github.com/amxv/dm-dot/pkg/message"
)

const helpText = `dm-dot - send a one-way message to your subscribed ChatGPT dot

Usage:
  dm-dot send [--json] [--url HTTPS_URL] MESSAGE...
  dm-dot send [--json] [--file PATH]
  printf 'message' | dm-dot send [--json]
  dm-dot configure --url HTTPS_URL --token-stdin
  dm-dot config show|clear
  dm-dot upgrade [--check]
  dm-dot version

The CLI sends via POST /v1/messages. ChatGPT webhook acceptance does not
mean your dot has processed the message. There is no reply channel.

Configuration (environment variables override saved config):
  DM_DOT_API_URL        HTTPS URL of YOUR deployed dm-dot API
  DM_DOT_REST_API_KEY   REST bearer token (not the MCP key)

Configure once to use dm-dot from scripts and local agents without exporting
credentials. The token is read from stdin, not command-line arguments.
Run 'dm-dot send --help' for send-specific usage.
`

func Run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, httpClient *http.Client) error {
	if len(args) == 0 {
		_, _ = io.WriteString(stdout, helpText)
		return nil
	}
	switch args[0] {
	case "--help", "-h", "help":
		_, _ = io.WriteString(stdout, helpText)
		return nil
	case "--version", "version":
		_, _ = fmt.Fprintf(stdout, "dm-dot v%s\n", buildinfo.Current())
		return nil
	case "send":
		return send(ctx, args[1:], stdin, stdout, stderr, httpClient)
	case "configure":
		return configure(args[1:], stdin, stdout, stderr)
	case "config":
		return showConfig(args[1:], stdout)
	case "upgrade":
		return upgrade.Run(ctx, args[1:], stdout)
	default:
		return fmt.Errorf("unknown command %q (run 'dm-dot --help')", args[0])
	}
}

func send(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer, httpClient *http.Client) error {
	flags := flag.NewFlagSet("send", flag.ContinueOnError)
	flags.SetOutput(stderr)
	jsonResult := flags.Bool("json", false, "print accepted event as JSON")
	filePath := flags.String("file", "", "read message from file instead of arguments")
	urlOverride := flags.String("url", "", "override the deployment URL")
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: dm-dot send [--json] [--url HTTPS_URL] [--file PATH] [MESSAGE...]")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	words := flags.Args()
	if *filePath != "" && len(words) > 0 {
		return errors.New("choose --file or message arguments, not both")
	}
	var input string
	if *filePath != "" {
		if *filePath == "-" {
			value, err := boundedRead(stdin)
			if err != nil {
				return err
			}
			input = value
		} else {
			file, err := os.Open(*filePath)
			if err != nil {
				return fmt.Errorf("open message file: %w", err)
			}
			defer file.Close()
			value, err := boundedRead(file)
			if err != nil {
				return err
			}
			input = value
		}
	} else if len(words) == 0 || (len(words) == 1 && words[0] == "-") {
		value, err := boundedRead(stdin)
		if err != nil {
			return err
		}
		input = value
	} else {
		input = strings.Join(words, " ")
	}
	cfg, err := config.Effective()
	if err != nil {
		return err
	}
	if *urlOverride != "" {
		cfg.APIURL = *urlOverride
	}
	result, err := (client.Client{APIURL: cfg.APIURL, Token: cfg.Token, HTTPClient: httpClient}).Send(ctx, input)
	if err != nil {
		return err
	}
	if *jsonResult {
		return json.NewEncoder(stdout).Encode(result)
	}
	_, _ = fmt.Fprintf(stdout, "ChatGPT accepted event %s for asynchronous processing by your dot.\n", result.EventID)
	return nil
}

func boundedRead(input io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(input, message.MaxRESTBodyBytes+1))
	if err != nil {
		return "", fmt.Errorf("read message: %w", err)
	}
	if len(data) > message.MaxRESTBodyBytes {
		return "", errors.New("message input exceeds 32 KiB")
	}
	return string(data), nil
}

func configure(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("configure", flag.ContinueOnError)
	flags.SetOutput(stderr)
	endpoint := flags.String("url", "", "your deployment HTTPS URL")
	tokenStdin := flags.Bool("token-stdin", false, "read the REST API key from standard input")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if len(flags.Args()) > 0 || !*tokenStdin || *endpoint == "" {
		return errors.New("usage: dm-dot configure --url HTTPS_URL --token-stdin")
	}
	if _, err := client.EndpointURL(*endpoint); err != nil {
		return err
	}
	data, err := io.ReadAll(io.LimitReader(stdin, 4097))
	if err != nil {
		return err
	}
	if len(data) > 4096 {
		return errors.New("REST key is too large")
	}
	key := strings.TrimSpace(string(data))
	if key == "" || strings.ContainsAny(key, " \n\r\t") {
		return errors.New("REST key must be a nonempty bearer token")
	}
	if err := config.Save(config.Settings{APIURL: *endpoint, Token: key}); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(stdout, "Saved dm-dot configuration for this user (token not displayed).")
	return nil
}

func showConfig(args []string, stdout io.Writer) error {
	if len(args) != 1 || (args[0] != "show" && args[0] != "clear") {
		return errors.New("usage: dm-dot config show|clear")
	}
	if args[0] == "clear" {
		if err := config.Clear(); err != nil {
			return err
		}
		_, _ = fmt.Fprintln(stdout, "Saved dm-dot configuration cleared.")
		return nil
	}
	cfg, err := config.Effective()
	if err != nil {
		return err
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "(not set)"
	} else if _, err := client.EndpointURL(cfg.APIURL); err != nil {
		cfg.APIURL = "(invalid deployment URL)"
	}
	_, _ = fmt.Fprintf(stdout, "API URL: %s\nREST key: %s\n", cfg.APIURL, map[bool]string{true: "configured", false: "not configured"}[cfg.Token != ""])
	return nil
}
