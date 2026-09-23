package core

import (
	"io"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/knadh/goyesql/v2"
	"github.com/knadh/listmonk/internal/dbconn"
	"github.com/knadh/listmonk/models"
	"github.com/lib/pq"
)

func TestValidateSQLiteQueryTables(t *testing.T) {
	db, err := dbconn.Open(dbconn.Config{Type: dbconn.SQLite, Path: t.TempDir() + "/query.db"})
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
	query := `SELECT subscribers.* FROM subscribers LEFT JOIN subscriber_lists ON subscriber_lists.subscriber_id=subscribers.id
		WHERE (json_array_length(listmonk_array($1))=0 OR subscriber_lists.list_id IN
		(SELECT value FROM json_each(listmonk_array($1)))) LIMIT $2 OFFSET $3`
	if err := validateQueryTables(db, query, allowedSubQueryTables, pq.Array([]int{}), 10, 0); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../queries-sqlite/subscribers.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := goyesql.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	dynamic := strings.ReplaceAll(queries["query-subscribers"].Query, "%query%", "TRUE")
	dynamic = strings.ReplaceAll(dynamic, "%order%", "subscribers.id DESC")
	if err := validateQueryTables(db, dynamic, allowedSubQueryTables, pq.Array([]int{}), "", "", 0, 20); err != nil {
		t.Fatal(err)
	}
	bad := `SELECT * FROM subscribers WHERE EXISTS (SELECT 1 FROM settings)`
	if err := validateQueryTables(db, bad, allowedSubQueryTables); err == nil {
		t.Fatal("expected settings table to be rejected")
	}
}

func TestQuerySubscribersSQLiteDoesNotHoldConnectionDuringLazyLoad(t *testing.T) {
	db, err := dbconn.Open(dbconn.Config{Type: dbconn.SQLite, Path: t.TempDir() + "/subscribers.db"})
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
	if _, err = db.Exec("INSERT INTO lists(id,uuid,name,type,tags) VALUES(1,'list','List','public','{}')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO subscribers(id,uuid,email,name,attribs) VALUES(1,'sub','person@example.com','Person',?)", []byte(`{}`)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO subscriber_lists(subscriber_id,list_id,status) VALUES(1,1,'confirmed')"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../queries-sqlite/subscribers.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := goyesql.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	lazy, err := db.Preparex(queries["get-subscriber-lists-lazy"].Query)
	if err != nil {
		t.Fatal(err)
	}
	core := &Core{db: db, q: &models.Queries{
		QuerySubscribers:       queries["query-subscribers"].Query,
		QuerySubscribersCount:  queries["query-subscribers-count"].Query,
		GetSubscriberListsLazy: &models.PreparedStatement{Stmt: lazy},
	}, log: log.New(io.Discard, "", 0)}
	done := make(chan error, 1)
	go func() { _, _, err := core.QuerySubscribers("", "", nil, "", "desc", "id", 0, 20); done <- err }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("subscriber query deadlocked while lazy loading lists")
	}
}
