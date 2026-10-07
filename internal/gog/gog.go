// Package gog shells out to the gog CLI (github.com/openclaw/gogcli) for Drive access.
package gog

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// MinVersion is the oldest gog release this tool is tested against.
var MinVersion = [3]int{0, 40, 0}

const (
	callTimeout   = 2 * time.Minute
	maxAttempts   = 4
	baseDelay     = 2 * time.Second
	exitTransport = 8 // gog's exit code for transport-level errors
)

// File is one Drive search result.
type File struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	MimeType     string `json:"mimeType"`
	ModifiedTime string `json:"modifiedTime"`
}

// Client runs gog commands.
type Client struct {
	Path    string    // gog binary
	Account string    // optional; empty uses gog's default account
	Log     io.Writer // retry notices
}

// exeDir returns the directory of the running binary, where release archives
// put the bundled gog. Tests override it.
var exeDir = func() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	return filepath.Dir(exe)
}

// Locate finds a gog that is at least MinVersion: the one on PATH if new
// enough, else the gog bundled next to this executable. Notices go to log.
func Locate(ctx context.Context, log io.Writer) (string, error) {
	if path, err := exec.LookPath("gog"); err == nil {
		err := checkVersion(ctx, path)
		if err == nil {
			return path, nil
		}
		fmt.Fprintf(log, "%v; trying the bundled gog\n", err)
	}
	name := "gog"
	if runtime.GOOS == "windows" {
		name = "gog.exe"
	}
	if dir := exeDir(); dir != "" {
		bundled := filepath.Join(dir, name)
		if _, err := os.Stat(bundled); err == nil {
			return bundled, checkVersion(ctx, bundled)
		}
	}
	return "", fmt.Errorf("gog %d.%d.%d or newer not found on PATH or next to gemini-notes-export; "+
		"install it from https://github.com/openclaw/gogcli/releases or reinstall gemini-notes-export from a release archive, which bundles gog",
		MinVersion[0], MinVersion[1], MinVersion[2])
}

func checkVersion(ctx context.Context, path string) error {
	out, err := exec.CommandContext(ctx, path, "--version").Output() // #nosec G204 -- path is a located gog binary
	if err != nil {
		return fmt.Errorf("running %s --version: %w", path, err)
	}
	v, err := ParseVersion(string(out))
	if err != nil {
		return err
	}
	if !AtLeast(v, MinVersion) {
		return fmt.Errorf("gog %d.%d.%d at %s is too old; need >= %d.%d.%d",
			v[0], v[1], v[2], path, MinVersion[0], MinVersion[1], MinVersion[2])
	}
	return nil
}

var versionRe = regexp.MustCompile(`v?(\d+)\.(\d+)\.(\d+)`)

// ParseVersion extracts the first semver triple from `gog --version` output,
// e.g. "v0.40.0 (a3065f53 2026-09-11T23:26:00Z)".
func ParseVersion(s string) ([3]int, error) {
	m := versionRe.FindStringSubmatch(s)
	if m == nil {
		return [3]int{}, fmt.Errorf("cannot parse gog version from %q", strings.TrimSpace(s))
	}
	var v [3]int
	for i := range v {
		v[i], _ = strconv.Atoi(m[i+1])
	}
	return v, nil
}

// AtLeast reports whether v >= min.
func AtLeast(v, min [3]int) bool {
	for i := range v {
		if v[i] != min[i] {
			return v[i] > min[i]
		}
	}
	return true
}

// run executes gog with the standard flags, a per-call timeout, and retries on exit code 8.
func (c *Client) run(ctx context.Context, args ...string) ([]byte, error) {
	args = append(args, "--json", "--no-input", "--readonly")
	if c.Account != "" {
		args = append(args, "-a", c.Account)
	}
	for attempt := 1; ; attempt++ {
		out, err := c.once(ctx, args)
		var exitErr *exec.ExitError
		if err == nil || !errors.As(err, &exitErr) || exitErr.ExitCode() != exitTransport || attempt >= maxAttempts {
			return out, err
		}
		delay := baseDelay << (attempt - 1)
		delay += time.Duration(rand.Int64N(int64(delay))) // #nosec G404 -- retry jitter, not security
		fmt.Fprintf(c.Log, "gog: transport error (attempt %d/%d), retrying in %s\n", attempt, maxAttempts, delay.Round(100*time.Millisecond))
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
}

func (c *Client) once(ctx context.Context, args []string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, c.Path, args...) // #nosec G204 -- fixed binary, args built here
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("gog %s: timed out after %s", args[0]+" "+args[1], callTimeout)
		}
		return nil, fmt.Errorf("gog %s %s: %w: %s", args[0], args[1], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// SearchNotes returns all Gemini notes docs modified after the given time, across all pages.
func (c *Client) SearchNotes(ctx context.Context, after time.Time) ([]File, error) {
	q := fmt.Sprintf("name contains 'Notes by Gemini' and mimeType = 'application/vnd.google-apps.document' and modifiedTime > '%s' and trashed = false",
		after.UTC().Format(time.RFC3339))
	var all []File
	page := ""
	for {
		args := []string{"drive", "search", "--raw-query", q, "--max=100"}
		if page != "" {
			args = append(args, "--page", page)
		}
		out, err := c.run(ctx, args...)
		if err != nil {
			return nil, err
		}
		var resp struct {
			Files         []File `json:"files"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(out, &resp); err != nil {
			return nil, fmt.Errorf("parsing gog drive search output: %w", err)
		}
		all = append(all, resp.Files...)
		if resp.NextPageToken == "" {
			return all, nil
		}
		page = resp.NextPageToken
	}
}

// ExportMarkdown exports a Google Doc as markdown to outPath (which must not exist).
func (c *Client) ExportMarkdown(ctx context.Context, id, outPath string) error {
	_, err := c.run(ctx, "docs", "export", id, "--format=md", "--out="+outPath)
	return err
}
