package badoption

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/common/xray/crypto"
	E "github.com/sagernet/sing/common/exceptions"
)

type Range struct {
	From int32 `json:"from"`
	To   int32 `json:"to"`
}

func (c *Range) Build() *Range {
	return (*Range)(c)
}

func (c *Range) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%d-%d", c.From, c.To))
}

func (c *Range) UnmarshalJSON(content []byte) error {
	var rangeValue struct {
		From int32 `json:"from"`
		To   int32 `json:"to"`
	}
	var stringValue string
	err := json.Unmarshal(content, &stringValue)
	if err == nil {
		parts := strings.Split(stringValue, "-")
		if len(parts) != 2 {
			return E.New("invalid length of range parts")
		}
		from, err := strconv.ParseInt(parts[0], 10, 32)
		if err != nil {
			return err
		}
		to, err := strconv.ParseInt(parts[1], 10, 32)
		if err != nil {
			return err
		}
		rangeValue.From, rangeValue.To = int32(from), int32(to)
	} else {
		err := json.Unmarshal(content, &rangeValue)
		if err != nil {
			return err
		}
	}
	if rangeValue.From > rangeValue.To {
		return E.New("invalid range")
	}
	*c = Range{rangeValue.From, rangeValue.To}
	return nil
}

func (c Range) Rand() int32 {
	return int32(crypto.RandBetween(int64(c.From), int64(c.To)))
}
