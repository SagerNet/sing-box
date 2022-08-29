package baderror

import (
	"context"
	"io"
	"net"
)

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
