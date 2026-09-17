package utils

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// BoolInt stores booleans as SMALLINT (0/1) in PostgreSQL and marshals as
// true/false in JSON. Mirrors the accounting services' convention so SSO
// account payloads (which use 0/1) parse cleanly.
type BoolInt bool

func (b BoolInt) ToInt() int {
	if b {
		return 1
	}
	return 0
}

func (b BoolInt) Value() (driver.Value, error) {
	if b {
		return int64(1), nil
	}
	return int64(0), nil
}

func (b *BoolInt) Scan(src any) error {
	switch v := src.(type) {
	case bool:
		*b = BoolInt(v)
	case int64:
		*b = v != 0
	case []byte:
		*b = len(v) > 0 && v[0] != '0' && v[0] != 'f' && v[0] != 'F'
	case string:
		*b = v != "0" && v != "false" && v != "f" && v != "F" && v != ""
	case nil:
		*b = false
	default:
		return fmt.Errorf("BoolInt: cannot scan type %T", src)
	}
	return nil
}

func (b BoolInt) MarshalJSON() ([]byte, error) {
	return json.Marshal(bool(b))
}

func (b *BoolInt) UnmarshalJSON(data []byte) error {
	var v any
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case bool:
		*b = BoolInt(val)
	case float64:
		*b = val != 0
	case string:
		*b = val != "0" && val != "false" && val != ""
	case nil:
		*b = false
	default:
		return fmt.Errorf("BoolInt: cannot unmarshal %T", v)
	}
	return nil
}
