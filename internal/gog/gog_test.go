package gog

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestVersion(t *testing.T) {
	tests := []struct {
		out  string
		want [3]int
		ok   bool
	}{
		{"v0.40.0 (a3065f53 2026-09-11T23:26:00Z)\n", [3]int{0, 40, 0}, true},
		{"v0.41.2 (deadbeef)", [3]int{0, 41, 2}, true},
		{"v0.39.9 (x)", [3]int{0, 39, 9}, false},
		{"v1.0.0", [3]int{1, 0, 0}, true},
		{"0.9.100", [3]int{0, 9, 100}, false},
	}
	for _, tt := range tests {
		v, err := ParseVersion(tt.out)
		if err != nil || v != tt.want {
			t.Errorf("ParseVersion(%q) = %v, %v; want %v", tt.out, v, err, tt.want)
		}
		if got := AtLeast(v, MinVersion); got != tt.ok {
			t.Errorf("AtLeast(%v) = %v, want %v", v, got, tt.ok)
		}
	}
	if _, err := ParseVersion("garbage"); err == nil {
		t.Error("ParseVersion(garbage) should fail")
	}
}

func fakeGog(t *testing.T, version string) string {
	t.Helper()
	dir := t.TempDir()
	script := "#!/bin/sh\necho '" + version + " (fake)'\n"
	if err := os.WriteFile(filepath.Join(dir, "gog"), []byte(script), 0o700); err != nil { // #nosec G306 -- test script must be executable
		t.Fatal(err)
	}
	return dir
}

func TestLocate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a shell-script fake gog")
	}
	newDir, oldDir, emptyDir := fakeGog(t, "v0.43.0"), fakeGog(t, "v0.39.0"), t.TempDir()
	defer func(f func() string) { exeDir = f }(exeDir)
	tests := []struct {
		name, path, exe, want string
	}{
		{"new gog on PATH wins", newDir, oldDir, filepath.Join(newDir, "gog")},
		{"old PATH gog falls back to bundled", oldDir, newDir, filepath.Join(newDir, "gog")},
		{"no PATH gog uses bundled", emptyDir, newDir, filepath.Join(newDir, "gog")},
		{"nothing found", emptyDir, emptyDir, ""},
		{"only old gogs", oldDir, oldDir, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PATH", tt.path)
			exeDir = func() string { return tt.exe }
			got, err := Locate(context.Background(), io.Discard)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("want error, got %s", got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("Locate = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}
