package daemon

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

func NewServer(startedService *StartedService, secret string) *grpc.Server {
	server := grpc.NewServer(
		grpc.ChainUnaryInterceptor(newUnaryAuthInterceptor(secret), UnaryErrorInterceptor),
		grpc.ChainStreamInterceptor(newStreamAuthInterceptor(secret), StreamErrorInterceptor),
	)
	healthServer := health.NewServer()
	RegisterStartedServiceServer(server, startedService)
	healthServer.SetServingStatus(StartedService_ServiceDesc.ServiceName, grpc_health_v1.HealthCheckResponse_SERVING)
	grpc_health_v1.RegisterHealthServer(server, healthServer)
	reflection.Register(server)
	return server
}

func newUnaryAuthInterceptor(secret string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		err := authenticate(ctx, secret)
		if err != nil {
			return nil, err
		}
		return handler(ctx, request)
	}
}

func newStreamAuthInterceptor(secret string) grpc.StreamServerInterceptor {
	return func(server any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		err := authenticate(stream.Context(), secret)
		if err != nil {
			return err
		}
		return handler(server, stream)
	}
}

func authenticate(ctx context.Context, secret string) error {
	if secret == "" {
		return nil
	}
	md, loaded := metadata.FromIncomingContext(ctx)
	if !loaded {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}
	values := md.Get("authorization")
	if len(values) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization")
	}
	token, isBearer := strings.CutPrefix(values[0], "Bearer ")
	if !isBearer || token != secret {
		return status.Error(codes.Unauthenticated, "invalid authorization")
	}
	return nil
}
