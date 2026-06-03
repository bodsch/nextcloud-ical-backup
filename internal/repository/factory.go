package repository

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql" // registers the "mysql" driver
	_ "modernc.org/sqlite"             // registers the pure-Go "sqlite" driver

	"bodsch.me/nextcloud-ical-backup/internal/ncconfig"
)

const defaultMySQLPort = 3306

// Open returns the repository matching the configured database backend. The
// caller owns the connection and must call Close.
func Open(cfg *ncconfig.Config) (Repository, error) {
	switch cfg.DBType {
	case ncconfig.SQLite:
		return openSQLite(cfg)
	case ncconfig.MySQL:
		return openMySQL(cfg)
	default:
		return nil, fmt.Errorf("unsupported database backend: %s", cfg.DBType)
	}
}

// openSQLite opens the configured SQLite database in read-only mode so a
// backup can never mutate the live database.
func openSQLite(cfg *ncconfig.Config) (Repository, error) {
	path, err := cfg.SQLiteDatabasePath()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("SQLite database not found at %s", path)
	}
	// Open read-only so a backup can never mutate the live database.
	db, err := sql.Open("sqlite", "file:"+path+"?mode=ro")
	if err != nil {
		return nil, err
	}
	return newSQLRepository(db, cfg.DBTablePrefix, cfg.Schema), nil
}

// openMySQL connects to the configured MySQL/MariaDB server and verifies the
// connection with a ping before returning.
func openMySQL(cfg *ncconfig.Config) (Repository, error) {
	port := cfg.DBPort
	if port == 0 {
		port = defaultMySQLPort
	}
	host := cfg.DBHost
	if host == "" {
		host = "localhost"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4",
		cfg.DBUser, cfg.DBPassword, host, port, cfg.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("cannot connect to MySQL/MariaDB at %s:%d: %w", host, port, err)
	}
	return newSQLRepository(db, cfg.DBTablePrefix, cfg.Schema), nil
}
