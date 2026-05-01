package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/dygo-dev/dygo/internal/config"
)

const version = "dev"

// Run executes the dygo command-line interface.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if ctx == nil {
		return fmt.Errorf("context is required")
	}
	if stdout == nil {
		return fmt.Errorf("stdout writer is required")
	}
	if stderr == nil {
		return fmt.Errorf("stderr writer is required")
	}

	if err := ctx.Err(); err != nil {
		return fmt.Errorf("run cli: %w", err)
	}

	if len(args) == 0 {
		printHelp(stdout)
		return nil
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return nil
	case "version":
		_, err := fmt.Fprintf(stdout, "dygo %s\n", version)
		if err != nil {
			return fmt.Errorf("write version: %w", err)
		}
		return nil
	case "serve":
		cfg := config.Default()
		_, err := fmt.Fprintf(stdout, "dygo serve will listen on %s\n", cfg.Server.Address())
		if err != nil {
			return fmt.Errorf("write serve output: %w", err)
		}
		return nil
	default:
		fmt.Fprintf(stderr, "unknown command: %s\n", args[0])
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func printHelp(w io.Writer) {
	lines := []string{
		"dygo is a metadata-driven business application platform.",
		"",
		"Usage:",
		"  dygo <command>",
		"",
		"Commands:",
		"  help      Show this help text",
		"  version   Print the dygo version",
		"  serve     Start the dygo server",
	}

	fmt.Fprintln(w, strings.Join(lines, "\n"))
}
