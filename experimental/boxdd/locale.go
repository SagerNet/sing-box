package main

import (
	"context"
	"slices"

	"github.com/sagernet/sing-box/experimental/locale"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func setLocaleFromContext(ctx context.Context) {
	requestMetadata, loaded := metadata.FromIncomingContext(ctx)
	if !loaded {
		return
	}
	slices.ContainsFunc(requestMetadata.Get("accept-language"), locale.Set)
}

func unaryLocaleInterceptor(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	setLocaleFromContext(ctx)
	return handler(ctx, request)
}

func streamLocaleInterceptor(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	setLocaleFromContext(stream.Context())
	return handler(server, stream)
}
