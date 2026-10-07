package syncer

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestFormatReport(t *testing.T) {
	tests := []struct {
		name    string
		changes map[string]map[string]bool
		want    string
	}{
		{"empty", map[string]map[string]bool{}, ""},
		{"mixed and sorted", map[string]map[string]bool{
			"2026-10-07": {"a": true, "b": true, "c": false},
			"2026-10-05": {"d": false},
			"2026-10-06": {"e": true},
		}, "2026-10-05  1 updated\n2026-10-06  1 new\n2026-10-07  2 new, 1 updated\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FormatReport(tt.changes); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")

	release, held, err := acquireLock(path)
	if err != nil || held {
		t.Fatalf("first acquire: held=%v err=%v", held, err)
	}
	if _, held, _ := acquireLock(path); !held {
		t.Error("second acquire should see a live holder")
	}
	release()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("release should remove the lock file")
	}

	// A dead PID is stale and gets replaced.
	os.WriteFile(path, []byte("999999999\n"), 0o600)
	release, held, err = acquireLock(path)
	if err != nil || held {
		t.Fatalf("stale acquire: held=%v err=%v", held, err)
	}
	b, _ := os.ReadFile(path)
	if string(b) != strconv.Itoa(os.Getpid())+"\n" {
		t.Errorf("lock holds %q", b)
	}
	release()
}

func TestReadCursor(t *testing.T) {
	dir := t.TempDir()
	if c, err := readCursor(filepath.Join(dir, "missing")); err != nil || !c.IsZero() {
		t.Errorf("missing cursor: %v %v", c, err)
	}
	p := filepath.Join(dir, "c")
	os.WriteFile(p, []byte("2026-10-07T19:43:21.889Z\n"), 0o600)
	c, err := readCursor(p)
	if err != nil || c.Nanosecond() != 889000000 {
		t.Errorf("cursor = %v, %v", c, err)
	}
}
