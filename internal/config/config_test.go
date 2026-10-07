package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolvePrecedence(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, "xdg"))
	t.Setenv(EnvOutputDir, "")
	t.Setenv(EnvAccount, "")

	// No config anywhere: clear error pointing at init.
	if _, err := Resolve(Config{}, ""); err == nil || !strings.Contains(err.Error(), "init") {
		t.Fatalf("want init error, got %v", err)
	}

	path, _ := DefaultPath()
	if path != filepath.Join(home, "xdg", "gemini-notes-export", "config.yaml") {
		t.Fatalf("DefaultPath = %s", path)
	}
	if err := Write(path, Config{OutputDir: "~/notes"}); err != nil {
		t.Fatal(err)
	}
	c, err := Resolve(Config{}, "")
	if err != nil || c.OutputDir != filepath.Join(home, "notes") || c.Account != "" {
		t.Fatalf("file only: %+v %v", c, err)
	}

	t.Setenv(EnvOutputDir, "/env/out")
	t.Setenv(EnvAccount, "env@example.com")
	c, _ = Resolve(Config{}, "")
	if c.OutputDir != "/env/out" || c.Account != "env@example.com" {
		t.Fatalf("env should beat file: %+v", c)
	}

	c, _ = Resolve(Config{OutputDir: "/flag/out", Account: "flag@example.com"}, "")
	if c.OutputDir != "/flag/out" || c.Account != "flag@example.com" {
		t.Fatalf("flags should beat env: %+v", c)
	}

	if _, err := Resolve(Config{}, filepath.Join(home, "missing.yaml")); err == nil {
		t.Fatal("explicit missing --config should fail")
	}
}

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "c.yaml")
	body := "# comment\n\noutput_dir: ~/My Notes  # trailing\naccount: 'it''s@example.com'\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil || c.OutputDir != "~/My Notes" || c.Account != "it's@example.com" {
		t.Fatalf("Load = %+v, %v", c, err)
	}

	// Round trip through Write, including a value that needs quoting.
	want := Config{OutputDir: `C:\notes #1`, Account: "a@example.com"}
	if err := Write(path, want); err != nil {
		t.Fatal(err)
	}
	if got, err := Load(path); err != nil || got != want {
		t.Fatalf("round trip = %+v, %v", got, err)
	}

	if err := os.WriteFile(path, []byte("outputdir: x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("unknown key should fail")
	}
}
