package option

import (
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

type TLSFragmentOptions struct {
	Enabled bool   `json:"enabled,omitempty"`
	Size    string `json:"size,omitempty"`  // Fragment size in Bytes
	Sleep   string `json:"sleep,omitempty"` // Time to sleep between sending the fragments in milliseconds
}

func ParseIntRange(str string) ([]uint64, error) {
	var err error
	result := make([]uint64, 2)

	splitString := strings.Split(str, "-")
	if len(splitString) == 2 {
		result[0], err = strconv.ParseUint(splitString[0], 10, 64)
		if err != nil {
			return nil, E.Cause(err, "Error parsing string to integer")
		}
		result[1], err = strconv.ParseUint(splitString[1], 10, 64)
		if err != nil {
			return nil, E.Cause(err, "Error parsing string to integer")
		}
	} else {
		result[0], err = strconv.ParseUint(splitString[0], 10, 64)
		if err != nil {
			return nil, E.Cause(err, "Error parsing string to integer")
		}
		result[1] = result[0]
	}
	return result, err
}
