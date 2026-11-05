package main

import (
	"context"
	"fmt"
	"os"

	"github.com/amxv/dm-dot/internal/cli"
)

func main() {
	if err := cli.Run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr, nil); err != nil {
		fmt.Fprintln(os.Stderr, "dm-dot:", err)
		os.Exit(1)
	}
}
