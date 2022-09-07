package baderror

import (
	"context"
	"io"
	"net"
	"strings"

	E "github.com/sagernet/sing/common/exceptions"
)

func Contains(err error, msgList ...string) bool {
	for _, msg := range msgList {
		if strings.Contains(err.Error(), msg) {
			return true
		}
	}
	return false
}

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

func WrapGRPC(err error) error {
	// grpc uses stupid internal error types
	if err == nil {
		return nil
	}
	if Contains(err, "EOF") {
		return io.EOF
	}
	if Contains(err, "Canceled") {
		return context.Canceled
	}
	if Contains(err,
		"the client connection is closing",
		"server closed the stream without sending trailers") {
		return net.ErrClosed
	}
	return err
}

func WrapQUIC(err error) error {
	if err == nil {
		return nil
	}
	if Contains(err, "canceled with error code 0") {
		return net.ErrClosed
	}
	return err
}
