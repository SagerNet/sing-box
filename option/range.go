package option

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

type IntRange struct {
	Min uint64
	Max uint64
}

func ParseIntRange(str string) ([]uint64, error) {
	var err error
	result := make([]uint64, 2)

	splitString := strings.Split(str, "-")
	if len(splitString) == 2 {
		result[0], err = strconv.ParseUint(splitString[0], 10, 64)
		if err != nil {
			return nil, E.Cause(err, "error parsing string to integer")
		}
		result[1], err = strconv.ParseUint(splitString[1], 10, 64)
		if err != nil {
			return nil, E.Cause(err, "error parsing string to integer")
		}

		if result[1] < result[0] {
			return nil, E.Cause(E.New(fmt.Sprintf("upper bound value (%d) must be greater than or equal to lower bound value (%d)", result[1], result[0])), "invalid range")
		}
	} else {
		result[0], err = strconv.ParseUint(splitString[0], 10, 64)
		if err != nil {
			return nil, E.Cause(err, "error parsing string to integer")
		}
		result[1] = result[0]
	}
	return result, err
}

// GetRandomIntFromRange generate a uniform random number given the range
func GetRandomIntFromRange(min uint64, max uint64) int64 {
	if max == 0 {
		return 0
	}
	if min == max {
		return int64(min)
	}
	randomInt, _ := rand.Int(rand.Reader, big.NewInt(int64(max-min)+1))
	return int64(min) + randomInt.Int64()
}
