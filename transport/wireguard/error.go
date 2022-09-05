package wireguard

import "net"

type wireError struct {
	cause error
}

func (w *wireError) Error() string {
	return w.cause.Error()
}

func (w *wireError) Timeout() bool {
	if cause, causeNet := w.cause.(net.Error); causeNet {
		return cause.Timeout()
	}
	return false
}

func (w *wireError) Temporary() bool {
	return true
}
