// Package syncer runs one sync: find changed Gemini notes in Drive, export,
// transform, and write them under the output directory.
package syncer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/chrisedwards/gemini-notes-export/internal/gog"
	"github.com/chrisedwards/gemini-notes-export/internal/notes"
)

const (
	cursorFile = ".gemini-notes-export-cursor"
	lockFile   = ".gemini-notes-export.lock"
	// ponytail: fixed 10-minute overlap re-reads docs Drive indexed late; a doc
	// whose modifiedTime lands more than 10 minutes behind the cursor is missed.
	// Make it configurable if that shows up in practice.
	overlap  = 10 * time.Minute
	docMime  = "application/vnd.google-apps.document"
	nameTail = "Notes by Gemini"
)

// Options are the inputs for one sync, already merged from flags, env, and config.
type Options struct {
	OutputDir string
	Since     time.Time // zero means "use the cursor"
	Account   string
}

// Run performs one sync. Changed dates go to stdout; everything else to stderr.
func Run(ctx context.Context, opts Options, stdout, stderr io.Writer) error {
	if err := os.MkdirAll(opts.OutputDir, 0o750); err != nil {
		return err
	}
	release, held, err := acquireLock(filepath.Join(opts.OutputDir, lockFile))
	if err != nil {
		return err
	}
	if held {
		fmt.Fprintln(stderr, "Another gemini-notes-export sync is running for this output dir; skipping.")
		return nil
	}
	defer release()

	cursorPath := filepath.Join(opts.OutputDir, cursorFile)
	cursor, err := readCursor(cursorPath)
	if err != nil {
		return err
	}
	start := opts.Since
	if start.IsZero() {
		if cursor.IsZero() {
			return fmt.Errorf("no sync cursor in %s yet; pass --since YYYY-MM-DD for the first run", opts.OutputDir)
		}
		start = cursor.Add(-overlap)
	}

	gogPath, err := gog.Locate(ctx)
	if err != nil {
		return err
	}
	client := &gog.Client{Path: gogPath, Account: opts.Account, Log: stderr}

	fmt.Fprintf(stderr, "Searching Drive for notes modified after %s\n", start.Format(time.RFC3339))
	found, err := client.SearchNotes(ctx, start)
	if err != nil {
		return err
	}
	var files []gog.File
	for _, f := range found {
		if f.MimeType == docMime && strings.HasSuffix(f.Name, nameTail) {
			files = append(files, f)
		}
	}
	// Oldest first, so if two docs share a name the newest wins.
	sort.SliceStable(files, func(i, j int) bool { return files[i].ModifiedTime < files[j].ModifiedTime })
	fmt.Fprintf(stderr, "Found %d notes docs\n", len(files))

	tmp, err := os.MkdirTemp("", "gemini-notes-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)

	maxSeen, maxSeenRaw := start, start.UTC().Format(time.RFC3339)
	changes := map[string]map[string]bool{} // date -> summary path -> isNew
	failed := 0
	for i, f := range files {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		status, date, err := syncDoc(ctx, client, f, opts.OutputDir, filepath.Join(tmp, strconv.Itoa(i)+".md"), stderr)
		if err != nil {
			failed++
			fmt.Fprintf(stderr, "  FAILED %s: %v\n", f.Name, err)
			continue
		}
		fmt.Fprintf(stderr, "  %-9s %s\n", status, f.Name)
		if status != "unchanged" {
			if changes[date] == nil {
				changes[date] = map[string]bool{}
			}
			key := filepath.Join(date, notes.FileName(f.Name))
			if _, seen := changes[date][key]; !seen {
				changes[date][key] = status == "new"
			}
		}
		if t, err := time.Parse(time.RFC3339Nano, f.ModifiedTime); err == nil && t.After(maxSeen) {
			maxSeen, maxSeenRaw = t, f.ModifiedTime
		}
	}

	fmt.Fprint(stdout, FormatReport(changes))

	if failed > 0 {
		return fmt.Errorf("%d of %d docs failed; cursor not advanced", failed, len(files))
	}
	if maxSeen.After(cursor) {
		if err := os.WriteFile(cursorPath, []byte(maxSeenRaw+"\n"), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// syncDoc exports one doc and writes its files. status is new, updated, or unchanged.
func syncDoc(ctx context.Context, client *gog.Client, f gog.File, outDir, tmpPath string, stderr io.Writer) (status, date string, err error) {
	if err := client.ExportMarkdown(ctx, f.ID, tmpPath); err != nil {
		return "", "", err
	}
	raw, err := os.ReadFile(tmpPath) // #nosec G304 -- our own temp file
	if err != nil {
		return "", "", err
	}
	modified, _ := time.Parse(time.RFC3339Nano, f.ModifiedTime)
	name := notes.FileName(f.Name)
	date = notes.DateFolder(name, modified)
	day, _ := time.ParseInLocation(time.DateOnly, date, time.Local)
	res := notes.Transform(string(raw), notes.TitleFromName(f.Name), day)
	if res.Warning != "" {
		fmt.Fprintf(stderr, "  warning: %s: %s\n", f.Name, res.Warning)
	}

	summaryPath := filepath.Join(outDir, date, name)
	_, statErr := os.Stat(summaryPath)
	existed := statErr == nil
	wroteSummary, err := writeIfChanged(summaryPath, res.Summary)
	if err != nil {
		return "", "", err
	}
	wroteFull, err := writeIfChanged(filepath.Join(outDir, date, "full-transcripts", name), res.Full)
	if err != nil {
		return "", "", err
	}
	switch {
	case !wroteSummary && !wroteFull:
		return "unchanged", date, nil
	case existed:
		return "updated", date, nil
	default:
		return "new", date, nil
	}
}

func writeIfChanged(path, content string) (bool, error) {
	old, err := os.ReadFile(path) // #nosec G304 -- path under the user's output dir
	if err == nil && string(old) == content {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, []byte(content), 0o600)
}

// FormatReport renders changed dates, sorted, as "YYYY-MM-DD  2 new, 1 updated".
func FormatReport(changes map[string]map[string]bool) string {
	dates := make([]string, 0, len(changes))
	for d := range changes {
		dates = append(dates, d)
	}
	sort.Strings(dates)
	var b strings.Builder
	for _, d := range dates {
		var nNew, nUpd int
		for _, isNew := range changes[d] {
			if isNew {
				nNew++
			} else {
				nUpd++
			}
		}
		var parts []string
		if nNew > 0 {
			parts = append(parts, fmt.Sprintf("%d new", nNew))
		}
		if nUpd > 0 {
			parts = append(parts, fmt.Sprintf("%d updated", nUpd))
		}
		fmt.Fprintf(&b, "%s  %s\n", d, strings.Join(parts, ", "))
	}
	return b.String()
}

func readCursor(path string) (time.Time, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path under the user's output dir
	if errors.Is(err, fs.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(b)))
	if err != nil {
		return time.Time{}, fmt.Errorf("bad cursor file %s: %w", path, err)
	}
	return t, nil
}

// acquireLock creates the lock file holding our PID. held is true when a live
// process already owns it. A lock left by a dead process is removed.
func acquireLock(path string) (release func(), held bool, err error) {
	for range 2 {
		f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600) // #nosec G304 -- path under the user's output dir
		if err == nil {
			_, err = fmt.Fprintf(f, "%d\n", os.Getpid())
			f.Close()
			if err != nil {
				os.Remove(path)
				return nil, false, err
			}
			return func() { os.Remove(path) }, false, nil
		}
		if !errors.Is(err, fs.ErrExist) {
			return nil, false, err
		}
		b, _ := os.ReadFile(path) // #nosec G304 -- path under the user's output dir
		if pid, _ := strconv.Atoi(strings.TrimSpace(string(b))); alive(pid) {
			return nil, true, nil
		}
		os.Remove(path)
	}
	return nil, false, fmt.Errorf("could not acquire lock %s", path)
}

// ponytail: Signal(0) liveness is Unix-only; on Windows every lock looks stale.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	p, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	err = p.Signal(syscall.Signal(0))
	return err == nil || errors.Is(err, syscall.EPERM)
}
