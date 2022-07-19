package clashapi

var (
	ErrUnauthorized   = newError("Unauthorized")
	ErrBadRequest     = newError("Body invalid")
	ErrForbidden      = newError("Forbidden")
	ErrNotFound       = newError("Resource not found")
	ErrRequestTimeout = newError("Timeout")
)

// HTTPError is custom HTTP error for API
type HTTPError struct {
	Message string `json:"message"`
}

func (e *HTTPError) Error() string {
	return e.Message
}

func newError(msg string) *HTTPError {
	return &HTTPError{Message: msg}
}
