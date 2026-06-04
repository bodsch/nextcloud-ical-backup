# Pre-Commit Hooks Setup

Dieses Projekt verwendet [pre-commit](https://pre-commit.com/) für automatisierte Code-Qualitätsprüfungen.

## Installation

### 1. Python installieren
Pre-commit benötigt Python. Falls nicht vorhanden:
```bash
# macOS
brew install python3

# Ubuntu/Debian
sudo apt-get install python3 python3-pip
```

### 2. Pre-Commit installieren
```bash
pip install pre-commit
```

### 3. Pre-Commit Hooks initialisieren
```bash
pre-commit install
```

Dies erstellt Git-Hooks im `.git/hooks` Verzeichnis.

## Getriggerte Hooks

Beim `git commit` werden automatisch folgende Checks ausgeführt:

- **gofmt**: Formatiert Go-Code
- **go vet**: Prüft auf häufige Fehler in Go-Code
- **go test**: Führt Tests aus
- **golangci-lint**: Erweiterte Linting-Checks
- **Allgemeine Hooks**: Trailing Whitespace, Dateigrößen, etc.

## Manuelle Ausführung

Falls du die Hooks manuell testen möchtest:

```bash
# Alle Hooks für alle Dateien ausführen
pre-commit run --all-files

# Nur spezifische Hook ausführen
pre-commit run gofmt --all-files
pre-commit run go-vet --all-files
pre-commit run go-test --all-files
```

## Hooks überspringen

Falls nötig (nicht empfohlen), kannst du Pre-Commit Hooks beim Commit mit dem Flag `--no-verify` überspringen:

```bash
git commit --no-verify
```

## Weitere Informationen

- [Pre-Commit Dokumentation](https://pre-commit.com/)
- `.pre-commit-config.yaml` - Konfigurationsdatei mit allen Hooks
