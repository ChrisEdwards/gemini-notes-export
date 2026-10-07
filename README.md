# gemini-notes-export

Exports Google Meet "Notes by Gemini" docs from your Google Drive into dated
markdown files: a short summary file and a full file with the transcript.
Runs well from cron and only prints the dates that changed, so other scripts
can pick up new notes.

It talks to Google through the [gog CLI](https://github.com/openclaw/gogcli)
and only reads: every call passes `--readonly`, and the setup below grants
read-only scopes.

## Install

macOS and Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/chrisedwards/gemini-notes-export/main/install.sh | sh
```

This installs `gemini-notes-export` and a bundled `gog` into `~/.local/bin`
(set `INSTALL_DIR=/some/dir` to change it). Rerun it to upgrade.

Manual: download the archive for your platform from
[Releases](https://github.com/chrisedwards/gemini-notes-export/releases) and put
both binaries in the same directory on your `PATH`. Windows uses the `.zip`.

From source (Go 1.25+, then install gog yourself):

```sh
go install github.com/chrisedwards/gemini-notes-export/cmd/gemini-notes-export@latest
```

### Bundled gog

`gemini-notes-export` uses the `gog` on your `PATH` if it is v0.40.0 or newer.
Otherwise it uses the `gog` next to its own executable (the one the release
archive ships). The installer skips the bundled `gog` when a same-or-newer
`gog` is already installed, so it never replaces yours.

## Setup

### 1. Create the config

```sh
gemini-notes-export init
```

It asks for the output directory and, optionally, a Google account email, and
writes `~/.config/gemini-notes-export/config.yaml`. Leave the account blank if
gog has only one account: gog then uses its default. `init --force` overwrites
an existing config.

### 2. Google auth setup

gog uses your own Google Cloud OAuth client, so no third party sees your data.

1. In the [Google Cloud console](https://console.cloud.google.com/), create or
   pick a project and enable the **Google Drive API** and **Google Docs API**.
2. Configure the OAuth consent screen (Internal for Workspace, or External with
   yourself as a test user).
3. Create an OAuth client ID of type **Desktop app** and download its JSON.
4. Register it with gog and authorize read-only access:

```sh
gog auth credentials set ~/Downloads/client_secret_XXXX.json
gog auth add you@example.com --services drive,docs --readonly
```

`--readonly` makes gog request read-only scopes. A browser window opens to
approve access.

### 3. First sync

```sh
gemini-notes-export sync --since 2026-09-01
```

Later runs continue from a saved cursor, so plain `gemini-notes-export sync`
is enough.

## Configuration

`~/.config/gemini-notes-export/config.yaml` (or
`$XDG_CONFIG_HOME/gemini-notes-export/config.yaml`):

```yaml
output_dir: "~/meeting-notes"
account: "you@example.com"   # optional; omit to use gog's default account
```

Each setting can also come from a flag or environment variable. Precedence:
flags, then env, then the config file.

| Setting    | Flag           | Environment variable              |
|------------|----------------|-----------------------------------|
| output_dir | `--output-dir` | `GEMINI_NOTES_EXPORT_OUTPUT_DIR`  |
| account    | `--account`    | `GEMINI_NOTES_EXPORT_ACCOUNT`     |

`--config path` reads a different config file. `~` in `output_dir` expands to
your home directory. gog also reads `XDG_CONFIG_HOME` for its own credentials, so
to move only this tool's config, use `--config` rather than changing
`XDG_CONFIG_HOME`.

## Usage

```sh
gemini-notes-export init [--force] [--config path]
gemini-notes-export sync [--since YYYY-MM-DD] [--output-dir dir] [--account email] [--config path]
gemini-notes-export version
```

`--since YYYY-MM-DD` means local midnight on that date and overrides the
cursor. The first sync into a new output directory needs it.

### stdout contract

stdout lists only the dates that changed, one per line, sorted:

```
2026-10-06  1 new
2026-10-07  2 new, 1 updated
```

No changes means empty stdout. Progress, warnings, and errors go to stderr.

### Exit codes

`0` on success, or when another sync already holds the lock. `1` on errors,
including any doc that failed to export (the others still sync; the cursor
stays put so the failed doc is retried next run).

### Cron

```cron
*/15 * * * * $HOME/.local/bin/gemini-notes-export sync >> $HOME/.cache/gemini-notes-export.log 2>&1
```

cron has a minimal `PATH`, so use the full path to the binary. The bundled
`gog` is found next to it.

## Output

For a doc named `Team Sync - 2026/10/06 15:00 EDT - Notes by Gemini`:

```
<output-dir>/2026-10-06/Team Sync - 2026_10_06 15_00 EDT - Notes by Gemini.md                   # summary
<output-dir>/2026-10-06/full-transcripts/Team Sync - 2026_10_06 15_00 EDT - Notes by Gemini.md  # full
```

- File name: the Drive name with `/ : < > " | ? *` replaced by `_`, plus `.md`.
- Date folder: the first `YYYY_MM_DD` in the name (meeting date in the
  organizer's time zone), else the local date of the doc's last modification.
- Summary: title, date, invitees, then the Next steps (when present) and
  Details sections.
- Full: title, date, invitees, every Full notes section (Summary, Decisions,
  Next steps, Details, ...), then the transcript.
- The Quick notes tab, survey and disclaimer lines, images, links, and markdown
  escapes are dropped or cleaned. Links keep their text.
- Files are rewritten only when their content changes.

### State files in the output dir

- `.gemini-notes-export-cursor`: the newest Drive `modifiedTime` synced. Each
  run searches from 10 minutes before it. It advances only when every doc in
  the run succeeded, and never moves backwards.
- `.gemini-notes-export.lock`: holds the PID of a running sync. A second run
  exits 0 with a note on stderr. A lock left by a dead process is removed. On
  Windows, where the PID can't be checked, a lock older than one hour counts as
  stale.

## Limitations

- The overlap window is a fixed 10 minutes. A doc whose Drive `modifiedTime`
  lands more than 10 minutes behind the cursor is missed (rerun with `--since`).
- Renaming a doc in Drive writes new files under the new name; the old files
  stay.
- Two docs with the same title and the same start minute map to the same file
  name; the most recently modified one wins.
- Only docs whose name ends in "Notes by Gemini" are exported.

## Development

```sh
make build        # ./gemini-notes-export
make check-test   # gofmt, go vet, golangci-lint, unit tests
```

Test fixtures in `internal/notes/testdata` are synthetic. Never commit real
meeting exports. Regenerate goldens with `go test ./internal/notes -update`
and review the diff.

Releases: pushing a `v*` tag runs `.github/workflows/release.yml`, which builds
all platforms and bundles the gog release pinned in `GOG_VERSION`, checked
against gogcli's `checksums.txt`.

## License

MIT. Release archives also include gog, which is MIT licensed by its authors
(see `LICENSE-gogcli` in the archive).
