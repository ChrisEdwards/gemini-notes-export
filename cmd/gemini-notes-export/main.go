// Command gemini-notes-export syncs Google Meet "Notes by Gemini" docs from
// Google Drive into dated markdown files, using the gog CLI.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/chrisedwards/gemini-notes-export/internal/syncer"
)

// Version information, injected at build time via ldflags.
var (
	Version   = "dev"
	Build     = "unknown"
	BuildTime = "unknown"
)

const usage = `Usage: gemini-notes-export <command> [flags]

Commands:
  sync      Export new or changed Gemini notes to markdown
  version   Print the version
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdout, os.Stderr)
	stop()
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return errors.New("missing command")
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "gemini-notes-export %s (build %s, %s)\n", Version, Build, BuildTime)
		return nil
	case "sync":
		opts, err := parseSync(args[1:], stderr)
		if err != nil {
			return err
		}
		return syncer.Run(ctx, opts, stdout, stderr)
	case "help", "-h", "--help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stderr, usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func parseSync(args []string, stderr io.Writer) (syncer.Options, error) {
	var opts syncer.Options
	var since string
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&opts.OutputDir, "output-dir", "", "directory to write dated notes folders into (required)")
	fs.StringVar(&since, "since", "", "export notes modified since this date (YYYY-MM-DD, local midnight); overrides the cursor")
	fs.StringVar(&opts.Account, "account", "", "Google account email for gog (default: gog's default account)")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() > 0 {
		return opts, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if opts.OutputDir == "" {
		return opts, errors.New("--output-dir is required")
	}
	if since != "" {
		t, err := time.ParseInLocation(time.DateOnly, since, time.Local)
		if err != nil {
			return opts, fmt.Errorf("--since must be YYYY-MM-DD: %w", err)
		}
		opts.Since = t
	}
	return opts, nil
}
