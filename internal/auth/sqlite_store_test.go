package auth

import (
	"os"
	"testing"

	"github.com/knadh/listmonk/internal/dbconn"
)

func TestSQLiteSessionStore(t *testing.T) {
	db, err := dbconn.Open(dbconn.Config{Type: dbconn.SQLite, Path: t.TempDir() + "/sessions.db"})
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

	store := newSQLiteSessionStore(db.DB)
	if err := store.Create("session"); err != nil {
		t.Fatal(err)
	}
	if err := store.SetMulti("session", map[string]any{"user_id": 42, "name": "Ada"}); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Int(store.Get("session", "user_id")); err != nil || got != 42 {
		t.Fatalf("user_id=%d err=%v", got, err)
	}
	if err := store.Delete("session", "name"); err != nil {
		t.Fatal(err)
	}
	if got, err := store.Get("session", "name"); err != nil || got != nil {
		t.Fatalf("name=%v err=%v", got, err)
	}
	if err := store.Destroy("session"); err != nil {
		t.Fatal(err)
	}
}
