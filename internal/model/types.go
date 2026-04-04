package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringArray is stored as JSON in the database (compatible with both SQLite and PostgreSQL).
type StringArray []string

func (a *StringArray) Scan(src interface{}) error {
	if src == nil {
		*a = StringArray{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case string:
		b = []byte(v)
	case []byte:
		b = v
	default:
		return fmt.Errorf("unsupported type for StringArray: %T", src)
	}
	var result []string
	if err := json.Unmarshal(b, &result); err != nil {
		*a = StringArray{}
		return nil
	}
	*a = result
	return nil
}

func (a StringArray) Value() (driver.Value, error) {
	if len(a) == 0 {
		return "[]", nil
	}
	b, err := json.Marshal([]string(a))
	if err != nil {
		return "[]", nil
	}
	return string(b), nil
}
