// Package config reads and writes ~/.config/gemini-notes-export/config.yaml and
// merges it with flags and environment variables.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Environment variables that override the config file.
const (
	EnvOutputDir = "GEMINI_NOTES_EXPORT_OUTPUT_DIR"
	EnvAccount   = "GEMINI_NOTES_EXPORT_ACCOUNT"
)

// Config is the user's saved settings.
type Config struct {
	OutputDir string
	Account   string // optional; empty uses gog's default account
}

// DefaultPath is $XDG_CONFIG_HOME/gemini-notes-export/config.yaml, falling back
// to ~/.config (also on macOS, where os.UserConfigDir would pick Library).
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "gemini-notes-export", "config.yaml"), nil
}

// Resolve merges flags > env > config file. path is the --config flag value;
// empty means DefaultPath, where a missing file is fine. A missing explicit
// path is an error. OutputDir comes back with ~ expanded and must be set.
func Resolve(flags Config, path string) (Config, error) {
	explicit := path != ""
	if !explicit {
		var err error
		if path, err = DefaultPath(); err != nil {
			return Config{}, err
		}
	}
	file, err := Load(path)
	if errors.Is(err, fs.ErrNotExist) && !explicit {
		err = nil
	}
	if err != nil {
		return Config{}, err
	}
	c := Config{
		OutputDir: first(flags.OutputDir, os.Getenv(EnvOutputDir), file.OutputDir),
		Account:   first(flags.Account, os.Getenv(EnvAccount), file.Account),
	}
	if c.OutputDir == "" {
		return Config{}, fmt.Errorf("no output directory set; run `gemini-notes-export init` or pass --output-dir (or set %s)", EnvOutputDir)
	}
	c.OutputDir, err = ExpandHome(c.OutputDir)
	return c, err
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// ExpandHome replaces a leading ~ with the user's home directory.
func ExpandHome(p string) (string, error) {
	if p != "~" && !strings.HasPrefix(p, "~/") && !strings.HasPrefix(p, `~\`) {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, p[1:]), nil
}

// Load reads a config file. A missing file returns an error wrapping fs.ErrNotExist.
// ponytail: flat `key: value` YAML subset (comments, quoted values), not a full
// YAML parser; swap in gopkg.in/yaml.v3 if the config ever nests.
func Load(path string) (Config, error) {
	f, err := os.Open(path) // #nosec G304 -- user-chosen config path
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	var c Config
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			return Config{}, fmt.Errorf("%s:%d: expected `key: value`", path, n)
		}
		val, err := parseValue(strings.TrimSpace(val))
		if err != nil {
			return Config{}, fmt.Errorf("%s:%d: %w", path, n, err)
		}
		switch strings.TrimSpace(key) {
		case "output_dir":
			c.OutputDir = val
		case "account":
			c.Account = val
		default:
			return Config{}, fmt.Errorf("%s:%d: unknown key %q (want output_dir or account)", path, n, key)
		}
	}
	return c, sc.Err()
}

func parseValue(v string) (string, error) {
	switch {
	case strings.HasPrefix(v, `"`):
		return strconv.Unquote(v)
	case strings.HasPrefix(v, "'"):
		if len(v) < 2 || !strings.HasSuffix(v, "'") {
			return "", errors.New("unterminated quote")
		}
		return strings.ReplaceAll(v[1:len(v)-1], "''", "'"), nil
	}
	if i := strings.Index(v, " #"); i >= 0 {
		v = v[:i]
	}
	return strings.TrimSpace(v), nil
}

// Write saves c to path, creating the parent directory.
func Write(path string, c Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# gemini-notes-export config. Flags and GEMINI_NOTES_EXPORT_* env vars override these.\n")
	fmt.Fprintf(&b, "output_dir: %s\n", strconv.Quote(c.OutputDir))
	if c.Account != "" {
		fmt.Fprintf(&b, "account: %s\n", strconv.Quote(c.Account))
	} else {
		b.WriteString("# account: \"you@example.com\"  # omit to use gog's default account\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}
