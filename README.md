# nextcloud-ical-backup (Go)

A Go port of the Python `nextcloud_ical_backup` utility. It backs up Nextcloud
calendars and contacts by reading the database directly, and restores calendars
through `occ`. The result is a single static binary with no runtime
dependencies (the SQLite driver is pure Go).

## Build

```bash
cd go
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
                                --backup-root ./backup --calendar Personal --dry-run
```

Configuration precedence matches the Python version:
built-in defaults < `--config <file.toml>` < CLI flags.

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
