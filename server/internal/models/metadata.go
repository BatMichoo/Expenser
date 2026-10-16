package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// Metadata is the raw JSON of an expense's nullable JSONB metadata column.
// Empty metadata is stored as NULL, and NULL reads back as empty.
type Metadata json.RawMessage

// Scan implements sql.Scanner.
func (m *Metadata) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*m = nil
	case []byte:
		*m = append((*m)[:0], v...)
	case string:
		*m = Metadata(v)
	default:
		return fmt.Errorf("cannot scan %T into Metadata", src)
	}
	return nil
}

// Value implements driver.Valuer.
func (m Metadata) Value() (driver.Value, error) {
	if len(m) == 0 {
		return nil, nil
	}
	return []byte(m), nil
}

// MarshalJSON keeps the metadata as raw JSON instead of a base64 string.
func (m Metadata) MarshalJSON() ([]byte, error) {
	if len(m) == 0 {
		return []byte("null"), nil
	}
	return m, nil
}

// UnmarshalJSON implements json.Unmarshaler.
func (m *Metadata) UnmarshalJSON(data []byte) error {
	*m = append((*m)[:0], data...)
	return nil
}
