package daemon

import (
	"context"
	"errors"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryErrorInterceptor(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	response, err := handler(ctx, request)
	if err != nil {
		return nil, mapStatusError(err)
	}
	return response, nil
}

func StreamErrorInterceptor(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	err := handler(server, stream)
	if err != nil {
		return mapStatusError(err)
	}
	return nil
}

func mapStatusError(err error) error {
	if _, loaded := status.FromError(err); loaded {
		return err
	}
	switch {
	case errors.Is(err, os.ErrInvalid):
		return status.Error(codes.FailedPrecondition, "service not started")
	case errors.Is(err, os.ErrClosed):
		return status.Error(codes.Unavailable, "service is closing")
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return status.FromContextError(err).Err()
	}
	return err
}
