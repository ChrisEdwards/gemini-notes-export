// Command gemini-notes-export syncs Google Meet "Notes by Gemini" docs from
// Google Drive into dated markdown files, using the gog CLI.
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/chrisedwards/gemini-notes-export/internal/config"
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
  init      Create the config file (output dir, optional Google account)
  sync      Export new or changed Gemini notes to markdown
  version   Print the version
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr)
	stop()
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
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
	case "init":
		return runInit(args[1:], stdin, stdout, stderr)
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
	var since, cfgPath string
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfgPath, "config", "", "config file (default: $XDG_CONFIG_HOME or ~/.config, then gemini-notes-export/config.yaml)")
	fs.StringVar(&opts.OutputDir, "output-dir", "", "directory to write dated notes folders into (overrides config)")
	fs.StringVar(&since, "since", "", "export notes modified since this date (YYYY-MM-DD, local midnight); overrides the cursor")
	fs.StringVar(&opts.Account, "account", "", "Google account email for gog (overrides config; default: gog's default account)")
	if err := fs.Parse(args); err != nil {
		return opts, err
	}
	if fs.NArg() > 0 {
		return opts, fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	cfg, err := config.Resolve(config.Config{OutputDir: opts.OutputDir, Account: opts.Account}, cfgPath)
	if err != nil {
		return opts, err
	}
	opts.OutputDir, opts.Account = cfg.OutputDir, cfg.Account
	if since != "" {
		t, err := time.ParseInLocation(time.DateOnly, since, time.Local)
		if err != nil {
			return opts, fmt.Errorf("--since must be YYYY-MM-DD: %w", err)
		}
		opts.Since = t
	}
	return opts, nil
}

func runInit(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	var cfgPath string
	var force bool
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&cfgPath, "config", "", "config file to write (default: $XDG_CONFIG_HOME or ~/.config, then gemini-notes-export/config.yaml)")
	fs.BoolVar(&force, "force", false, "overwrite an existing config file")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("unexpected arguments: %v", fs.Args())
	}
	if cfgPath == "" {
		var err error
		if cfgPath, err = config.DefaultPath(); err != nil {
			return err
		}
	}
	if _, err := os.Stat(cfgPath); err == nil && !force {
		return fmt.Errorf("%s already exists; use --force to overwrite", cfgPath)
	}

	in := bufio.NewReader(stdin)
	ask := func(prompt, def string) (string, error) {
		fmt.Fprint(stdout, prompt)
		line, err := in.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		if line = strings.TrimSpace(line); line == "" {
			return def, nil
		}
		return line, nil
	}
	var c config.Config
	var err error
	if c.OutputDir, err = ask("Output directory for notes [~/meeting-notes]: ", "~/meeting-notes"); err != nil {
		return err
	}
	if c.Account, err = ask("Google account email (blank = gog's default account): ", ""); err != nil {
		return err
	}
	if err := config.Write(cfgPath, c); err != nil {
		return err
	}
	acct := c.Account
	if acct == "" {
		acct = "you@example.com"
	}
	fmt.Fprintf(stdout, `
Wrote %s

Next steps:
  1. Create a Google Cloud OAuth client (Desktop app), download its JSON, then:
       gog auth credentials set /path/to/client_secret.json
  2. Authorize read-only Drive and Docs access:
       gog auth add %s --services drive,docs --readonly
  3. Export notes, starting from a date:
       gemini-notes-export sync --since YYYY-MM-DD
`, cfgPath, acct)
	return nil
}
