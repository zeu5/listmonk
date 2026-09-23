package models

import (
	"database/sql"
	"errors"

	"github.com/jmoiron/sqlx"
)

// PreparedStatement exposes the underlying database/sql statement for callers
// that bind PostgreSQL prepared operations to an explicit batch transaction.
type PreparedStatement struct{ *sqlx.Stmt }

func (s *PreparedStatement) SQLStmt() *sql.Stmt { return s.Stmt.Stmt }

// FuncStatement adapts a backend-specific operation to Statement.
type FuncStatement struct {
	ExecFunc   func(args ...any) (sql.Result, error)
	GetFunc    func(dest any, args ...any) error
	SelectFunc func(dest any, args ...any) error
}

func (s *FuncStatement) Exec(args ...any) (sql.Result, error) {
	if s.ExecFunc == nil {
		return nil, errors.New("operation does not support Exec")
	}
	return s.ExecFunc(args...)
}
func (s *FuncStatement) Get(dest any, args ...any) error {
	if s.GetFunc == nil {
		return errors.New("operation does not support Get")
	}
	return s.GetFunc(dest, args...)
}
func (s *FuncStatement) Select(dest any, args ...any) error {
	if s.SelectFunc == nil {
		return errors.New("operation does not support Select")
	}
	return s.SelectFunc(dest, args...)
}
