// Scalar types as nullable

package gofastersql

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// NullInherit is the structure that all other Null structures inherit from
type NullInherit struct {
	IsNull bool
}

// NullableTypes is the list of types that can be nulled
type NullableTypes interface {
	uint8 | uint16 | uint32 | uint64 | int8 | int16 | int32 | int64 | float32 | float64 | bool | string | []byte | sql.RawBytes | time.Time
}

// NullType is a generic nulled type. Inherits from NullInherit and contains the typed value
type NullType[T NullableTypes] struct {
	NullInherit
	Val T
}

// String converts a NullType into a user readable string. The Time format is 2006-01-02 15:04:05.99999.
func (t NullType[T]) String() string {
	if t.IsNull {
		return "NULL"
	}

	switch v := any(t.Val).(type) {
	case string:
		return v
	case []byte:
		return b2s(v)
	case sql.RawBytes:
		return b2s(v)
	case time.Time:
		return v.Format(`2006-01-02 15:04:05.99999`)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// MarshalJSON converts a NullType into JSON. The Time format is "2006-01-02T15:04:05.000Z".
func (t NullType[T]) MarshalJSON() ([]byte, error) {
	if t.IsNull {
		return []byte("null"), nil
	}

	var outStr string
	switch v := any(t.Val).(type) {
	case time.Time:
		return []byte(v.Format(`"2006-01-02T15:04:05.000Z"`)), nil
	case string:
		outStr = v
	case []byte:
		outStr = b2s(v)
	case sql.RawBytes:
		outStr = b2s(v)
	default:
		return []byte(fmt.Sprintf("%v", v)), nil
	}

	//JSON-ify a string
	newStr, _ := json.Marshal(outStr)
	return newStr, nil
}
