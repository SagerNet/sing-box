package daemon

import (
	"context"

	"github.com/sagernet/sing-box/experimental/locale"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

func contextWithLocale(ctx context.Context) context.Context {
	requestMetadata, loaded := metadata.FromIncomingContext(ctx)
	if !loaded {
		return ctx
	}
	for _, localeID := range requestMetadata.Get("accept-language") {
		localizedContext, matched := locale.ContextWithLocale(ctx, localeID)
		if matched {
			return localizedContext
		}
	}
	return ctx
}

func UnaryLocaleInterceptor(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	return handler(contextWithLocale(ctx), request)
}

func StreamLocaleInterceptor(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
	return handler(server, &localeServerStream{
		ServerStream: stream,
		ctx:          contextWithLocale(stream.Context()),
	})
}

func UnaryClientLocaleInterceptor(ctx context.Context, method string, request, reply any, connection *grpc.ClientConn, invoker grpc.UnaryInvoker, options ...grpc.CallOption) error {
	ctx = metadata.AppendToOutgoingContext(ctx, "accept-language", locale.FromContext(ctx).Locale)
	return invoker(ctx, method, request, reply, connection, options...)
}

func StreamClientLocaleInterceptor(ctx context.Context, description *grpc.StreamDesc, connection *grpc.ClientConn, method string, streamer grpc.Streamer, options ...grpc.CallOption) (grpc.ClientStream, error) {
	ctx = metadata.AppendToOutgoingContext(ctx, "accept-language", locale.FromContext(ctx).Locale)
	return streamer(ctx, description, connection, method, options...)
}

type localeServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *localeServerStream) Context() context.Context {
	return s.ctx
}
