package baderror

import (
	"io"
	"net"

	E "github.com/sagernet/sing/common/exceptions"
)

func WrapH2(err error) error {
	if err == nil {
		return nil
	}
	err = E.Unwrap(err)
	if err == io.ErrUnexpectedEOF {
		return io.EOF
	}
	if Contains(err, "client disconnected", "body closed by handler") {
		return net.ErrClosed
	}
	return err
}
