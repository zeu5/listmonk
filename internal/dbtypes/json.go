// Package dbtypes contains database-neutral SQL value types.
package dbtypes

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// RawJSON is a json.RawMessage that also implements sql.Scanner. PostgreSQL
// drivers normally return JSON as []byte, while SQLite commonly returns TEXT
// expressions as string. Supporting both here prevents query-specific scan
// behavior from leaking into models.
type RawJSON json.RawMessage

func (j *RawJSON) Scan(src any) error {
	switch value := src.(type) {
	case nil:
		*j = nil
	case []byte:
		*j = append((*j)[:0], value...)
	case string:
		*j = append((*j)[:0], value...)
	default:
		return fmt.Errorf("cannot scan %T into RawJSON", src)
	}
	return nil
}

func (j RawJSON) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	if !json.Valid(j) {
		return nil, fmt.Errorf("invalid JSON")
	}
	return []byte(j), nil
}

func (j RawJSON) MarshalJSON() ([]byte, error) {
	if j == nil {
		return []byte("null"), nil
	}
	return j, nil
}

func (j *RawJSON) UnmarshalJSON(data []byte) error {
	*j = append((*j)[:0], data...)
	return nil
}

func (j RawJSON) Bytes() []byte { return []byte(j) }
