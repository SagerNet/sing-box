package v2raykcp

import "errors"

var (
	// ErrIOTimeout is returned when I/O operation times out
	ErrIOTimeout = errors.New("i/o timeout")
	// ErrClosedListener is returned when listener is closed
	ErrClosedListener = errors.New("listener closed")
	// ErrClosedConnection is returned when connection is closed
	ErrClosedConnection = errors.New("connection closed")
)

func newError(values ...interface{}) error {
	return errors.New(toString(values...))
}

func toString(values ...interface{}) string {
	result := ""
	for _, value := range values {
		switch v := value.(type) {
		case string:
			result += v
		case error:
			result += v.Error()
		}
	}
	return result
}
