package dbconn

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/knadh/goyesql/v2"
	"github.com/knadh/listmonk/internal/dbops"
	"github.com/lib/pq"
)

func TestDriver(t *testing.T) {
	for _, tt := range []struct {
		in, want string
		err      bool
	}{
		{"", Postgres, false},
		{"postgresql", Postgres, false},
		{"POSTGRES", Postgres, false},
		{"sqlite", SQLite, false},
		{"mysql", "", true},
	} {
		got, err := (Config{Type: tt.in}).Driver()
		if (err != nil) != tt.err || got != tt.want {
			t.Fatalf("Driver(%q) = %q, %v; want %q, error=%v", tt.in, got, err, tt.want, tt.err)
		}
	}
}

func TestSQLiteConnection(t *testing.T) {
	db, err := Open(Config{Type: SQLite, Path: t.TempDir() + "/listmonk.db"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	var foreignKeys int
	if err := db.Get(&foreignKeys, "PRAGMA foreign_keys"); err != nil {
		t.Fatal(err)
	}
	if foreignKeys != 1 {
		t.Fatalf("foreign_keys = %d; want 1", foreignKeys)
	}
	if _, err := db.Exec("CREATE TABLE parent (id INTEGER PRIMARY KEY); CREATE TABLE child (parent_id INTEGER REFERENCES parent(id))"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO child(parent_id) VALUES (1)"); err == nil {
		t.Fatal("expected SQLite foreign-key enforcement")
	}
}

func TestSQLiteRequiresPath(t *testing.T) {
	if _, err := Open(Config{Type: SQLite}); err == nil {
		t.Fatal("expected missing db.path error")
	}
}

func TestSQLiteArrayCompatibility(t *testing.T) {
	db, err := Open(Config{Type: SQLite, Path: t.TempDir() + "/listmonk.db"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var got []string
	if err := db.Select(&got, "SELECT value FROM json_each(listmonk_array(?)) ORDER BY key", pq.Array([]string{"one", "two,three", "four"})); err != nil {
		t.Fatal(err)
	}
	want := []string{"one", "two,three", "four"}
	if len(got) != len(want) {
		t.Fatalf("values = %#v; want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("values = %#v; want %#v", got, want)
		}
	}
	var nested string
	if err := db.Get(&nested, "SELECT listmonk_array(?)", pq.Array([][]string{{"list:get", "list:manage"}, {"list:get", ""}})); err != nil {
		t.Fatal(err)
	}
	if nested != `[["list:get","list:manage"],["list:get",""]]` {
		t.Fatalf("nested array = %s", nested)
	}
}

func TestPreparePortedSQLiteQueries(t *testing.T) {
	db, err := Open(Config{Type: SQLite, Path: t.TempDir() + "/listmonk.db"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("../../schema-sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	files, err := filepath.Glob("../../queries-sqlite/*.sql")
	if err != nil {
		t.Fatal(err)
	}
	overrides := dbops.SQLite(db)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		queries, err := goyesql.ParseBytes(data)
		if err != nil {
			t.Fatal(err)
		}
		for name, query := range queries {
			if _, ok := overrides[name]; ok {
				continue
			}
			statement := strings.ReplaceAll(query.Query, "%order%", "1")
			if strings.HasPrefix(name, "get-campaign-analytics-") {
				statement = strings.ReplaceAll(statement, "%s", "campaign_views")
			} else {
				statement = strings.ReplaceAll(statement, "%s", "*")
			}
			if strings.Contains(name, "-by-query") {
				statement = strings.ReplaceAll(statement, "%query%", "SELECT id FROM subscribers")
			} else {
				statement = strings.ReplaceAll(statement, "%query%", "1")
			}
			if _, err := db.Prepare(statement); err != nil {
				t.Errorf("prepare %s: %v", name, err)
			}
		}
	}
}

func TestSQLiteSchema(t *testing.T) {
	db, err := Open(Config{Type: SQLite, Path: t.TempDir() + "/listmonk.db"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("../../schema-sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		t.Fatalf("install SQLite schema: %v", err)
	}
	var settings int
	if err := db.Get(&settings, "SELECT count(*) FROM settings"); err != nil || settings == 0 {
		t.Fatalf("settings count = %d, err=%v", settings, err)
	}
	var dashboard string
	if err := db.Get(&dashboard, "SELECT data FROM mat_dashboard_counts"); err != nil {
		t.Fatalf("read dashboard view: %v", err)
	}
}

func TestSQLiteUpdateSettingsStoresJSONValues(t *testing.T) {
	db, err := Open(Config{Type: SQLite, Path: t.TempDir() + "/settings.db"})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	schema, err := os.ReadFile("../../schema-sqlite.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(string(schema)); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../queries-sqlite/misc.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := goyesql.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}

	if _, err = db.Exec("UPDATE settings SET value=? WHERE key=?", "http://localhost:9000", "app.root_url"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("UPDATE settings SET value=? WHERE key=?", "0", "app.enable_public_archive"); err != nil {
		t.Fatal(err)
	}
	var recovered string
	if err = db.Get(&recovered, queries["get-settings"].Query); err != nil {
		t.Fatal(err)
	}
	var recoveredSettings map[string]any
	if err = json.Unmarshal([]byte(recovered), &recoveredSettings); err != nil {
		t.Fatal(err)
	}
	if recoveredSettings["app.root_url"] != "http://localhost:9000" {
		t.Fatalf("recovered app.root_url = %#v", recoveredSettings["app.root_url"])
	}
	if recoveredSettings["app.enable_public_archive"] != false {
		t.Fatalf("recovered app.enable_public_archive = %#v", recoveredSettings["app.enable_public_archive"])
	}

	in := []byte(`{
		"app.root_url": "http://localhost:9000",
		"app.message_rate": 25,
		"app.enable_public_archive": false,
		"app.notify_emails": ["admin@example.com"],
		"security.oidc": {"enabled": true, "default_user_role_id": null},
		"smtp": [{"enabled": true, "host": "smtp.example.com", "port": 587}]
	}`)
	if _, err = db.Exec(queries["update-settings"].Query, in); err != nil {
		t.Fatal(err)
	}

	var raw string
	if err = db.Get(&raw, queries["get-settings"].Query); err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err = json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if got["app.root_url"] != "http://localhost:9000" {
		t.Fatalf("app.root_url = %#v", got["app.root_url"])
	}
	if got["app.enable_public_archive"] != false {
		t.Fatalf("app.enable_public_archive = %#v", got["app.enable_public_archive"])
	}
}
