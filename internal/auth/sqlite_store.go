package auth

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/zerodha/simplesessions/v3"
)

// sqliteSessionStore implements simplesessions.Store using portable JSON
// serialization. Mutations are performed transactionally in Go because SQLite
// has no JSONB concatenation or deletion operators.
type sqliteSessionStore struct {
	db  *sql.DB
	ttl time.Duration
}

func newSQLiteSessionStore(db *sql.DB) *sqliteSessionStore {
	return &sqliteSessionStore{db: db, ttl: 24 * time.Hour}
}
func (s *sqliteSessionStore) Create(id string) error {
	_, err := s.db.Exec("INSERT INTO sessions(id, data) VALUES(?, '{}')", id)
	return err
}
func (s *sqliteSessionStore) GetAll(id string) (map[string]any, error) {
	var raw string
	err := s.db.QueryRow("SELECT data FROM sessions WHERE id=? AND created_at>=datetime('now', ?)", id, fmt.Sprintf("-%d seconds", int(s.ttl.Seconds()))).Scan(&raw)
	if err == sql.ErrNoRows {
		return nil, simplesessions.ErrInvalidSession
	}
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, err
	}
	return out, nil
}
func (s *sqliteSessionStore) Get(id, key string) (any, error) {
	m, e := s.GetAll(id)
	if e != nil {
		return nil, e
	}
	return m[key], nil
}
func (s *sqliteSessionStore) GetMulti(id string, keys ...string) (map[string]any, error) {
	m, e := s.GetAll(id)
	if e != nil {
		return nil, e
	}
	out := map[string]any{}
	for _, k := range keys {
		out[k] = m[k]
	}
	return out, nil
}
func (s *sqliteSessionStore) mutate(id string, fn func(map[string]any)) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var raw string
	if err = tx.QueryRow("SELECT data FROM sessions WHERE id=?", id).Scan(&raw); err == sql.ErrNoRows {
		return simplesessions.ErrInvalidSession
	}
	if err != nil {
		return err
	}
	m := map[string]any{}
	if err = json.Unmarshal([]byte(raw), &m); err != nil {
		return err
	}
	fn(m)
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE sessions SET data=? WHERE id=?", string(b), id); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *sqliteSessionStore) Set(id, key string, v any) error {
	return s.mutate(id, func(m map[string]any) { m[key] = v })
}
func (s *sqliteSessionStore) SetMulti(id string, v map[string]any) error {
	return s.mutate(id, func(m map[string]any) {
		for k, x := range v {
			m[k] = x
		}
	})
}
func (s *sqliteSessionStore) Delete(id string, keys ...string) error {
	return s.mutate(id, func(m map[string]any) {
		for _, k := range keys {
			delete(m, k)
		}
	})
}
func (s *sqliteSessionStore) Clear(id string) error {
	return s.mutate(id, func(m map[string]any) { clear(m) })
}
func (s *sqliteSessionStore) Destroy(id string) error {
	r, e := s.db.Exec("DELETE FROM sessions WHERE id=?", id)
	if e != nil {
		return e
	}
	n, _ := r.RowsAffected()
	if n == 0 {
		return simplesessions.ErrInvalidSession
	}
	return nil
}
func (s *sqliteSessionStore) Prune() error {
	_, e := s.db.Exec("DELETE FROM sessions WHERE created_at<datetime('now', ?)", fmt.Sprintf("-%d seconds", int(s.ttl.Seconds())))
	return e
}
func storeValue(v any, e error) (any, error) {
	if e != nil {
		return nil, e
	}
	if v == nil {
		return nil, simplesessions.ErrNil
	}
	return v, nil
}
func (s *sqliteSessionStore) Int(v any, e error) (int, error) {
	x, e := storeValue(v, e)
	if e != nil {
		return 0, e
	}
	n, ok := x.(float64)
	if !ok {
		return 0, simplesessions.ErrAssertType
	}
	return int(n), nil
}
func (s *sqliteSessionStore) Int64(v any, e error) (int64, error) {
	n, e := s.Int(v, e)
	return int64(n), e
}
func (s *sqliteSessionStore) UInt64(v any, e error) (uint64, error) {
	n, e := s.Int(v, e)
	return uint64(n), e
}
func (s *sqliteSessionStore) Float64(v any, e error) (float64, error) {
	x, e := storeValue(v, e)
	if e != nil {
		return 0, e
	}
	n, ok := x.(float64)
	if !ok {
		return 0, simplesessions.ErrAssertType
	}
	return n, nil
}
func (s *sqliteSessionStore) String(v any, e error) (string, error) {
	x, e := storeValue(v, e)
	if e != nil {
		return "", e
	}
	n, ok := x.(string)
	if !ok {
		return "", simplesessions.ErrAssertType
	}
	return n, nil
}
func (s *sqliteSessionStore) Bytes(v any, e error) ([]byte, error) {
	x, e := s.String(v, e)
	return []byte(x), e
}
func (s *sqliteSessionStore) Bool(v any, e error) (bool, error) {
	x, e := storeValue(v, e)
	if e != nil {
		return false, e
	}
	n, ok := x.(bool)
	if !ok {
		return false, simplesessions.ErrAssertType
	}
	return n, nil
}
