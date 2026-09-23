package auth

import (
	"os"
	"testing"
	"time"

	"github.com/knadh/goyesql/v2"
	"github.com/knadh/listmonk/internal/dbconn"
	"github.com/lib/pq"
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

func TestSQLiteUserAndRoleJSONScanning(t *testing.T) {
	db, err := dbconn.Open(dbconn.Config{Type: dbconn.SQLite, Path: t.TempDir() + "/users.db"})
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

	userPerms := pq.Array([]string{"users:get"})
	listPerms := pq.Array([]string{"lists:get"})
	if _, err = db.Exec("INSERT INTO lists(id,uuid,name,type) VALUES(1,'list-1','List 1','private')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO roles(id,type,name,permissions) VALUES(1,'user','Super Admin',?),(2,'list','List role',?)", userPerms, listPerms); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO roles(parent_id,list_id,type,permissions) VALUES(2,1,'list',?)", listPerms); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO users(username,password_login,email,name,type,user_role_id,list_role_id,status) VALUES('admin',1,'admin@example.com','Admin','user',1,2,'enabled')"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile("../../queries-sqlite/users.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := goyesql.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	var users []User
	if err = db.Select(&users, queries["get-users"].Query); err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0].ListsPermsRaw == nil {
		t.Fatalf("users=%#v", users)
	}
	var user User
	if err = db.Get(&user, queries["get-user"].Query, 1, "", ""); err != nil {
		t.Fatal(err)
	}
	if user.ListsPermsRaw == nil {
		t.Fatal("expected list role permissions")
	}

	data, err = os.ReadFile("../../queries-sqlite/roles.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err = goyesql.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	var roles []Role
	if err = db.Select(&roles, queries["get-user-roles"].Query, 1); err != nil {
		t.Fatal(err)
	}
	if len(roles) != 1 || len(roles[0].ListsRaw) == 0 {
		t.Fatalf("roles=%#v", roles)
	}
}

func TestSQLiteLoginThenSessionDoesNotDeadlock(t *testing.T) {
	db, err := dbconn.Open(dbconn.Config{Type: dbconn.SQLite, Path: t.TempDir() + "/login.db"})
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
	if _, err = db.Exec("INSERT INTO roles(id,type,name,permissions) VALUES(1,'user','Super Admin','{}')"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(`INSERT INTO users(username,password_login,password,email,name,type,user_role_id,status)
		VALUES('admin',1,listmonk_password('supersecret',''),'admin@example.com','Admin','user',1,'enabled')`); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("../../queries-sqlite/users.sql")
	if err != nil {
		t.Fatal(err)
	}
	queries, err := goyesql.ParseBytes(data)
	if err != nil {
		t.Fatal(err)
	}

	done := make(chan error, 1)
	go func() {
		var user User
		if err := db.Get(&user, queries["login-user"].Query, "admin", "supersecret"); err != nil {
			done <- err
			return
		}
		store := newSQLiteSessionStore(db.DB)
		if err := store.Create("login-session"); err != nil {
			done <- err
			return
		}
		done <- store.SetMulti("login-session", map[string]any{"user_id": user.ID, "oidc_token": ""})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SQLite login and session creation deadlocked")
	}
}
