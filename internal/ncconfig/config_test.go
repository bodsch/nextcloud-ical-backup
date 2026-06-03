package ncconfig

import (
	"os"
	"path/filepath"
	"testing"
)

const sqliteConfig = `<?php
$CONFIG = array (
  'dbtype' => 'sqlite3',
  'dbname' => 'nextcloud',
  'datadirectory' => '/var/www/nextcloud/data',
  'dbtableprefix' => 'oc_',
  'version' => '28.0.1.1',
);
`

const mysqlConfig = `<?php
$CONFIG = array (
  'dbtype' => 'mysql',
  'dbname' => 'nextcloud',
  'dbhost' => 'db.example.net:3307',
  'dbuser' => 'ncuser',
  'dbpassword' => 'secret',
  'version' => '22.2.0.0',
  'overwrite.cli.url' => 'https://cloud.example.net',
);
`

func write(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.php")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestSQLiteConfigAppendsDBSuffix(t *testing.T) {
	cfg, err := FromPHP(write(t, sqliteConfig))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBType != SQLite {
		t.Errorf("DBType = %s", cfg.DBType)
	}
	path, err := cfg.SQLiteDatabasePath()
	if err != nil {
		t.Fatal(err)
	}
	if path != "/var/www/nextcloud/data/nextcloud.db" {
		t.Errorf("sqlite path = %q (must append .db)", path)
	}
	if !cfg.Schema.FilterSubscriptionCache || !cfg.Schema.HasTrashbin {
		t.Errorf("schema = %+v", cfg.Schema)
	}
}

func TestMySQLConfigParsesHostPortAndURL(t *testing.T) {
	cfg, err := FromPHP(write(t, mysqlConfig))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DBType != MySQL || cfg.DBHost != "db.example.net" || cfg.DBPort != 3307 {
		t.Errorf("host/port wrong: %+v", cfg)
	}
	if cfg.DBUser != "ncuser" || cfg.DBPassword != "secret" {
		t.Errorf("credentials wrong: %+v", cfg)
	}
	if cfg.NextcloudURL != "https://cloud.example.net" {
		t.Errorf("url = %q", cfg.NextcloudURL)
	}
}

func TestMissingFileErrors(t *testing.T) {
	if _, err := FromPHP(filepath.Join(t.TempDir(), "nope.php")); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestUnsupportedDBTypeErrors(t *testing.T) {
	bad := `<?php $CONFIG = array ( 'dbtype' => 'oracle', );`
	if _, err := FromPHP(write(t, bad)); err == nil {
		t.Fatal("expected error for unsupported dbtype")
	}
}

func TestSchemaFromVersion(t *testing.T) {
	cases := []struct {
		version      string
		subscription bool
		trashbin     bool
	}{
		{"14.0.0", false, false},
		{"15.0.0", true, false},
		{"22.0.0", true, true},
	}
	for _, c := range cases {
		s := SchemaFromVersion(c.version)
		if s.FilterSubscriptionCache != c.subscription || s.HasTrashbin != c.trashbin {
			t.Errorf("SchemaFromVersion(%s) = %+v", c.version, s)
		}
	}
}

func TestDBTypeAcceptsMariaDB(t *testing.T) {
	got, err := DBTypeFromConfig("mariadb")
	if err != nil || got != MySQL {
		t.Errorf("mariadb => %s, %v", got, err)
	}
}
