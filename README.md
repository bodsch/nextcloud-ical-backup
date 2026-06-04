# nextcloud-ical-backup (Go)

A Go port of the Python `nextcloud_ical_backup` utility. It backs up Nextcloud
calendars and contacts by reading the database directly, and restores calendars
through `occ`. The result is a single static binary with no runtime
dependencies (the SQLite driver is pure Go).

## Build

```bash

export GOPATH="$HOME/src/go"
export GOMODCACHE=$GOPATH/pkg/mod

go build -o nextcloud-ical-backup ./cmd/nextcloud-ical-backup
```

`nextcloud-ical-backup --version` reports the version, build date and Go
toolchain. A plain `go build` reports `dev` / `unknown`; the release pipeline
stamps real values via ldflags:

```bash
go build -ldflags "-X main.version=v0.2.0 -X main.buildDate=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -o nextcloud-ical-backup ./cmd/nextcloud-ical-backup
```

## Usage

```bash
# Per-entry backup (default): one file per event/contact
./nextcloud-ical-backup backup --config-php /var/www/nextcloud/config/config.php \
                               --backup-root ./backup

# Aggregate backup: one combined file per calendar/addressbook
./nextcloud-ical-backup backup --nextcloud-path /var/www/nextcloud --aggregate

# List only, restricted to one user
./nextcloud-ical-backup backup --nextcloud-path /var/www/nextcloud --user alice --list-only

# Restore (dry-run) – auto-detects both backup layouts
./nextcloud-ical-backup restore --nextcloud-path /var/www/nextcloud \
                                --backup-root ./backup/2026-06-04 --calendar Personal --dry-run
```

Configuration precedence matches the Python version:
built-in defaults < `--config <file.toml>` < CLI flags.

## Backup tree layout

Each backup run writes into a **dated subdirectory** of `--backup-root`
(`YYYY-MM-DD`), so consecutive runs are kept side by side instead of
overwriting each other. Beneath the date, the tree is grouped by user and item
type:

```text
backup/
└── 2026-06-04/                 # one directory per backup day
    └── bodsch/                 # one directory per user
        ├── ics/                # calendars
        │   └── Personal/       # per-entry layout: one .ics per event
        │       ├── <uid-1>.ics
        │       └── <uid-2>.ics
        └── vcf/                # addressbooks
            └── Contacts/       # per-entry layout: one .vcf per contact
                └── <uid>.vcf
```

In **aggregate** mode (`--aggregate`) each calendar/addressbook is a single
combined file instead of a directory, e.g. `ics/Personal.ics`.

## Restore

Restore is a deliberately manual, explicit step — there is no auto-selection of
the "latest" backup. Point `--backup-root` at the level that **contains the
user directories**, i.e. the dated directory of the run you want to restore:

```bash
./nextcloud-ical-backup restore \
  --nextcloud-path /var/www/nextcloud \
  --backup-root ./backup/2026-06-04 \
  --user bodsch --calendar Personal --dry-run
```

The discovery expects exactly the layout produced by `backup` and resolves it
relative to `--backup-root`:

```text
<backup-root>/<user>/ics/<calendar>/*.ics   (or ics/<calendar>.ics in aggregate mode)
<backup-root>/<user>/vcf/<addressbook>/*.vcf
```

If you point `--backup-root` deeper (e.g. directly at `ics/Personal/`), the
entries there are misinterpreted as user directories and **nothing is found**.

Reading the report output:

```text
calendar: bodsch/Personal (4 file(s))
Restored 1 calendar(s), skipped 0 item(s).
```

- **1 calendar** counts the restored calendar (`Personal`).
- **4 file(s)** are the per-event `.ics` files that make up that one calendar;
  in per-entry mode they are all imported into the same calendar.

Notes:

- Use `--list-only` to print the discovered targets without invoking `occ` —
  handy for confirming the layout before a real restore.
- Use `--dry-run` to run the full selection but skip the `occ` import.
- Filter with repeatable `--user`, `--calendar` and `--addressbook` flags;
  omit them to restore everything found.
- **Addressbooks are skipped** on restore: stock Nextcloud `occ` has no contact
  import. They are reported under "skipped".

## Layout

```text
internal/domain      core models (CalendarItem, AddressbookItem, BackupFilter, BackupReport)
internal/ncconfig    config.php parser + version/schema profile
internal/repository  Repository interface + database/sql impl (sqlite + mysql) + factory
internal/service     ical/vcard builders, backup service, restore service
internal/settings    layered settings loader (defaults < TOML < CLI)
internal/util        helpers (filenames, principal URIs, UID extraction, CRLF writer)
cmd/nextcloud-ical-backup  CLI (stdlib flag, subcommand dispatch)
```

## Dependencies

- `modernc.org/sqlite` – pure-Go SQLite driver (no cgo, fully static binary)
- `github.com/go-sql-driver/mysql` – MySQL/MariaDB driver
- `github.com/BurntSushi/toml` – TOML configuration files

Both database drivers use `?` placeholders, so a single `database/sql`
repository serves both backends.

## Development

All commands are run from the `go/` directory.

```bash
gofmt -l .
go vet ./...
go test ./...
```

### Tests

```bash
# Run all tests
go test ./...

# Verbose, without the build/test cache (as the CI does)
go test ./... -count=1 -v

# With the race detector
go test ./... -race

# With a coverage summary
go test ./... -cover

# Run the tests of a single package
go test ./internal/service/...
```

## Pipelines (Forgejo Actions)

Two separate workflows:

- **`.forgejo/workflows/ci.yml`** — format, vet, test and a smoke build. Runs on
  every push to `main`, on pull/merge requests, and on manual dispatch. Publishes
  nothing.
- **`.forgejo/workflows/release.yml`** — builds static binaries and uploads them
  to the Forgejo **generic** package registry. Manual only (`workflow_dispatch`)
  and only when dispatched on a release tag:

```text
PUT {server}/api/packages/{owner}/generic/nextcloud-ical-backup/{version}/{file}
```

Built targets: linux/amd64 and linux/arm64 (each with a `.sha256` checksum;
darwin/windows are present but commented out). `CGO_ENABLED=0` plus the pure-Go
SQLite driver yields fully static binaries. The version and build date are
stamped into the binary via ldflags.

**Setup:** add a repo/org secret `FORGEJO_TOKEN` (a token with the
`write:package` scope). The job authenticates as `${{ github.actor }}`.

**Publish a release:**

1. Create and push the tag: `git tag v0.2.0 && git push origin v0.2.0`
2. In the Forgejo UI run the `release` workflow, choosing that tag under
   *"Use workflow from"*. (Triggering it on a branch fails by design.)

**Download a published binary:**

```bash
curl -L -o nextcloud-ical-backup \
  "https://forgejo.example.com/api/packages/<owner>/generic/nextcloud-ical-backup/v0.2.0/nextcloud-ical-backup_v0.2.0_linux_amd64"
```

Note: the generic registry rejects re-uploading an existing file (HTTP 409);
delete the package version first to republish the same tag.
