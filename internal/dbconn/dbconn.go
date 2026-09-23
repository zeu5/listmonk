// Package dbconn configures listmonk's supported SQL database connections.
package dbconn

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
	"modernc.org/sqlite"
)

const (
	Postgres = "postgres"
	SQLite   = "sqlite"
)

func init() {
	// Existing listmonk call sites use pq.Array, whose driver value is a
	// PostgreSQL array literal. SQLite queries pass those values through this
	// function before json_each(), avoiding backend conditionals at every call.
	sqlite.MustRegisterDeterministicScalarFunction("listmonk_array", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) == 0 || args[0] == nil {
				return "[]", nil
			}
			raw := fmt.Sprint(args[0])
			if json.Valid([]byte(raw)) {
				return raw, nil
			}
			if strings.HasPrefix(raw, "{{") {
				nested, err := parseNestedArray(raw)
				if err != nil {
					return nil, err
				}
				out, err := json.Marshal(nested)
				if err != nil {
					return nil, err
				}
				return string(out), nil
			}
			var values pq.StringArray
			if err := values.Scan(raw); err != nil {
				return nil, fmt.Errorf("decode array parameter: %w", err)
			}
			out, err := json.Marshal([]string(values))
			if err != nil {
				return nil, err
			}
			return string(out), nil
		})
	sqlite.MustRegisterScalarFunction("listmonk_password", 2,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			password := fmt.Sprint(args[0])
			hash := fmt.Sprint(args[1])
			if hash != "" {
				if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil {
					return hash, nil
				}
				return "", nil
			}
			encoded, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			return string(encoded), err
		})
	sqlite.MustRegisterDeterministicScalarFunction("listmonk_pg_array", 1,
		func(_ *sqlite.FunctionContext, args []driver.Value) (driver.Value, error) {
			if len(args) == 0 || args[0] == nil {
				return "{}", nil
			}
			var values []string
			if err := json.Unmarshal([]byte(fmt.Sprint(args[0])), &values); err != nil {
				return nil, err
			}
			return pq.Array(values).Value()
		})
}

func parseNestedArray(raw string) ([][]string, error) {
	var out [][]string
	start, depth := -1, 0
	quoted, escaped := false, false
	for i, r := range raw {
		if escaped {
			escaped = false
			continue
		}
		if r == '\\' && quoted {
			escaped = true
			continue
		}
		if r == '"' {
			quoted = !quoted
			continue
		}
		if quoted {
			continue
		}
		switch r {
		case '{':
			depth++
			if depth == 2 {
				start = i
			}
		case '}':
			if depth == 2 && start >= 0 {
				var row pq.StringArray
				if err := row.Scan(raw[start : i+1]); err != nil {
					return nil, fmt.Errorf("decode nested array parameter: %w", err)
				}
				out = append(out, []string(row))
				start = -1
			}
			depth--
		}
	}
	if depth != 0 || quoted {
		return nil, fmt.Errorf("decode nested array parameter: malformed array")
	}
	return out, nil
}

// Config contains database connection and pool settings. PostgreSQL fields are
// ignored for SQLite; Path is ignored for PostgreSQL.
type Config struct {
	Type        string        `koanf:"type"`
	Path        string        `koanf:"path"`
	Host        string        `koanf:"host"`
	Port        int           `koanf:"port"`
	User        string        `koanf:"user"`
	Password    string        `koanf:"password"`
	DBName      string        `koanf:"database"`
	SSLMode     string        `koanf:"ssl_mode"`
	Params      string        `koanf:"params"`
	MaxOpen     int           `koanf:"max_open"`
	MaxIdle     int           `koanf:"max_idle"`
	MaxLifetime time.Duration `koanf:"max_lifetime"`
}

// Driver returns the normalized database backend. An empty type preserves the
// historical PostgreSQL default.
func (c Config) Driver() (string, error) {
	driver := strings.ToLower(strings.TrimSpace(c.Type))
	if driver == "" || driver == "postgresql" {
		driver = Postgres
	}
	if driver != Postgres && driver != SQLite {
		return "", fmt.Errorf("unsupported database type %q (expected postgres or sqlite)", c.Type)
	}
	return driver, nil
}

// Open connects to the configured database and applies backend-appropriate
// pool settings.
func Open(c Config) (*sqlx.DB, error) {
	driver, err := c.Driver()
	if err != nil {
		return nil, err
	}

	if driver == SQLite {
		path := strings.TrimSpace(c.Path)
		if path == "" {
			return nil, fmt.Errorf("db.path is required when db.type is sqlite")
		}
		// Pragmas are part of the DSN so they are applied to every connection.
		dsn := path + "?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)"
		db, err := sqlx.Connect(SQLite, dsn)
		if err != nil {
			return nil, err
		}
		// A single connection avoids SQLITE_BUSY writer contention and is also
		// required for consistent :memory: database behavior.
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		db.SetConnMaxLifetime(0)
		return db.Unsafe(), nil
	}

	fields := map[string]string{
		"host": c.Host, "port": strconv.Itoa(c.Port), "user": c.User,
		"password": c.Password, "dbname": c.DBName, "sslmode": c.SSLMode,
	}
	if c.Port == 0 {
		delete(fields, "port")
	}
	parts := make([]string, 0, len(fields)+1)
	for key, value := range fields {
		if value != "" {
			parts = append(parts, key+"="+value)
		}
	}
	if c.Params != "" {
		parts = append(parts, c.Params)
	}
	db, err := sqlx.Connect(Postgres, strings.Join(parts, " "))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(c.MaxOpen)
	db.SetMaxIdleConns(c.MaxIdle)
	db.SetConnMaxLifetime(c.MaxLifetime)
	return db.Unsafe(), nil
}
