# gemini-notes-export

Syncs Google Meet "Notes by Gemini" docs from Google Drive into dated markdown
files. It shells out to the [gog CLI](https://github.com/openclaw/gogcli)
(v0.40.0 or newer, on `PATH`, already authenticated) and only reads from Google
(every call passes `--readonly`).

## Status

Early. Done: `sync` and `version`. Not yet: config file, `init`, environment
variables, installer, release workflow, bundled gog.

## Usage

```sh
make build

# First run: pick a start date.
./gemini-notes-export sync --output-dir ~/meeting-notes --since 2026-09-01

# Later runs continue from the saved cursor.
./gemini-notes-export sync --output-dir ~/meeting-notes

# Use a specific gog account instead of gog's default.
./gemini-notes-export sync --output-dir ~/meeting-notes --account you@example.com
```

`--since YYYY-MM-DD` means local midnight on that date and overrides the cursor.

## Output

For a doc named `Team Sync - 2026/10/06 15:00 EDT - Notes by Gemini`:

```
<output-dir>/2026-10-06/Team Sync - 2026_10_06 15_00 EDT - Notes by Gemini.md                   # summary
<output-dir>/2026-10-06/full-transcripts/Team Sync - 2026_10_06 15_00 EDT - Notes by Gemini.md  # full
```

- File name: the Drive name with `/ : < > " | ? *` replaced by `_`, plus `.md`.
- Date folder: the first `YYYY_MM_DD` in the name (meeting date in the
  organizer's time zone), else the local date of the doc's last modification.
- Summary: title, date, invitees, then the Next steps (when present) and Details sections.
- Full: title, date, invitees, every Full notes section (Summary, Decisions,
  Next steps, Details, ...), then the transcript.
- The Quick notes tab, survey and disclaimer lines, images, links, and markdown
  escapes are dropped or cleaned. Links keep their text.
- Files are rewritten only when their content changes.

stdout lists only the dates that changed, one per line, for scripting:

```
2026-10-06  1 new
2026-10-07  2 new, 1 updated
```

Progress and warnings go to stderr. No changes means empty stdout.

## State files in the output dir

- `.gemini-notes-export-cursor`: the newest Drive `modifiedTime` synced. Each
  run searches from 10 minutes before it. It advances only when every doc in the
  run succeeded, and never moves backwards.
- `.gemini-notes-export.lock`: holds the PID of a running sync. A second run
  exits 0 with a note on stderr. A lock left by a dead process is removed.

## Exit codes

`0` on success or when another sync holds the lock. `1` on errors, including
any doc that failed to export (the others still sync; the cursor stays put).

## Development

```sh
make check-test   # gofmt, go vet, golangci-lint, unit tests
```

Test fixtures in `internal/notes/testdata` are synthetic. Never commit real
meeting exports. Regenerate goldens with `go test ./internal/notes -update`
and review the diff.
