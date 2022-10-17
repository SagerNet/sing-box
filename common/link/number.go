package link

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// number supports json unmarshaling from number or string
type number int64

// UnmarshalJSON implements json.Unmarshaler
func (i *number) UnmarshalJSON(b []byte) error {
	var v interface{}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	switch value := v.(type) {
	case string:
		switch value {
		case "", "null":
			*i = 0
		default:
			var err error
			v, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return err
			}
			*i = number(v)
		}
	case float64:
		*i = number(value)
	default:
		return fmt.Errorf("invalid var int: %v", v)
	}
	return nil
}
