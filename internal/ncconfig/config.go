// Package ncconfig parses the relevant values from a Nextcloud config.php file.
//
// Parsing is regex based (instead of executing PHP) to avoid running untrusted
// code and to keep the dependency footprint minimal.
package ncconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// DatabaseType is a supported Nextcloud database backend.
type DatabaseType string

const (
	// SQLite is the SQLite backend.
	SQLite DatabaseType = "sqlite3"
	// MySQL is the MySQL/MariaDB backend.
	MySQL DatabaseType = "mysql"
)

// Fallback values used when a key is absent from config.php.
const (
	defaultTablePrefix  = "oc_"
	defaultSQLiteDBName = "owncloud"
	defaultVersion      = "9.1.0"
)

// DBTypeFromConfig maps a dbtype config value to a supported backend.
func DBTypeFromConfig(raw string) (DatabaseType, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "sqlite", "sqlite3":
		return SQLite, nil
	case "mysql", "mariadb":
		return MySQL, nil
	default:
		return "", fmt.Errorf("unsupported database type %q (only sqlite3 and mysql/mariadb are supported)", raw)
	}
}

// SchemaProfile captures version dependent schema traits.
type SchemaProfile struct {
	MajorVersion int
	// FilterSubscriptionCache is true for Nextcloud >= 15, where cached webcal
	// subscription objects (calendartype = 1) must be excluded.
	FilterSubscriptionCache bool
	// HasTrashbin is true for Nextcloud >= 22, which added the deleted_at column.
	HasTrashbin bool
}

// SchemaFromVersion derives schema traits from a dotted version string.
func SchemaFromVersion(version string) SchemaProfile {
	major := 9
	head := version
	if idx := strings.IndexByte(version, '.'); idx >= 0 {
		head = version[:idx]
	}
	if v, err := strconv.Atoi(head); err == nil {
		major = v
	}
	return SchemaProfile{
		MajorVersion:            major,
		FilterSubscriptionCache: major >= 15,
		HasTrashbin:             major >= 22,
	}
}

// Config holds the relevant values read from config.php.
type Config struct {
	ConfigPath    string
	DataDirectory string
	DBType        DatabaseType
	DBHost        string
	DBPort        int // 0 means "not configured"
	DBName        string
	DBUser        string
	DBPassword    string
	DBTablePrefix string
	Version       string
	Schema        SchemaProfile
	NextcloudURL  string
}

// Patterns matching PHP array assignments inside the $CONFIG array:
// scalarPattern matches quoted string values, literalPattern boolean/numeric ones.
var (
	scalarPattern  = regexp.MustCompile(`['"]([^'"]+)['"]\s*=>\s*['"]((?s:.*?))['"]\s*,`)
	literalPattern = regexp.MustCompile(`(?i)['"]([^'"]+)['"]\s*=>\s*(true|false|null|\d+)\s*,`)
)

// FromPHP parses a Nextcloud config.php file.
func FromPHP(configPath string) (*Config, error) {
	abs, err := filepath.Abs(expandUser(configPath))
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("nextcloud config.php not found at %s: %w", abs, err)
	}
	values := parse(string(data))

	dbType, err := DBTypeFromConfig(orDefault(values["dbtype"], "sqlite3"))
	if err != nil {
		return nil, err
	}
	host, port := splitHost(orDefault(values["dbhost"], "localhost"))
	version := orDefault(values["version"], defaultVersion)

	return &Config{
		ConfigPath:    abs,
		DataDirectory: resolveDataDirectory(abs, values["datadirectory"]),
		DBType:        dbType,
		DBHost:        host,
		DBPort:        port,
		DBName:        orDefault(values["dbname"], defaultSQLiteDBName),
		DBUser:        values["dbuser"],
		DBPassword:    values["dbpassword"],
		DBTablePrefix: orDefault(values["dbtableprefix"], defaultTablePrefix),
		Version:       version,
		Schema:        SchemaFromVersion(version),
		NextcloudURL:  values["overwrite.cli.url"],
	}, nil
}

// SQLiteDatabasePath returns the absolute path to the SQLite database file,
// which Nextcloud stores as <datadirectory>/<dbname>.db.
func (c *Config) SQLiteDatabasePath() (string, error) {
	if c.DBType != SQLite {
		return "", fmt.Errorf("SQLite database path is only available for sqlite3 installations")
	}
	name := c.DBName
	if !strings.HasSuffix(name, ".db") {
		name += ".db"
	}
	if filepath.IsAbs(name) {
		return name, nil
	}
	return filepath.Join(c.DataDirectory, name), nil
}

// parse extracts scalar and literal "'key' => value" assignments from the PHP
// config source. Scalar (quoted) values win over literal ones for a given key.
func parse(content string) map[string]string {
	values := map[string]string{}
	for _, m := range scalarPattern.FindAllStringSubmatch(content, -1) {
		values[strings.TrimSpace(m[1])] = strings.TrimSpace(m[2])
	}
	for _, m := range literalPattern.FindAllStringSubmatch(content, -1) {
		key := strings.TrimSpace(m[1])
		if _, ok := values[key]; !ok {
			values[key] = strings.TrimSpace(m[2])
		}
	}
	return values
}

// resolveDataDirectory resolves the data directory to an absolute path,
// defaulting to "<config parent>/../data" and resolving relative values against
// the config.php directory.
func resolveDataDirectory(configPath, raw string) string {
	if raw == "" {
		return filepath.Join(filepath.Dir(filepath.Dir(configPath)), "data")
	}
	if !filepath.IsAbs(raw) {
		return filepath.Join(filepath.Dir(configPath), raw)
	}
	return filepath.Clean(raw)
}

// splitHost splits a "host:port" value into a host and an integer port,
// returning port 0 when no valid port is present.
func splitHost(dbhost string) (string, int) {
	dbhost = strings.TrimSpace(dbhost)
	if strings.Count(dbhost, ":") == 1 {
		host, portText, _ := strings.Cut(dbhost, ":")
		if port, err := strconv.Atoi(portText); err == nil {
			return host, port
		}
	}
	return dbhost, 0
}

// expandUser expands a leading "~" to the current user's home directory.
func expandUser(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(path, "~"))
		}
	}
	return path
}

// orDefault returns value, or fallback when value is empty.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
