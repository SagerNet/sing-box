package v2raygrpc

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	GunService_Tun_FullMethodName = "/transport.v2raygrpc.GunService/Tun"
)

// GunServiceClient is the client API for GunService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type GunServiceClient interface {
	Tun(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[Hunk, Hunk], error)
}

type gunServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewGunServiceClient(cc grpc.ClientConnInterface) GunServiceClient {
	return &gunServiceClient{cc}
}

func (c *gunServiceClient) Tun(ctx context.Context, opts ...grpc.CallOption) (grpc.BidiStreamingClient[Hunk, Hunk], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &GunService_ServiceDesc.Streams[0], GunService_Tun_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[Hunk, Hunk]{ClientStream: stream}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type GunService_TunClient = grpc.BidiStreamingClient[Hunk, Hunk]

// GunServiceServer is the server API for GunService service.
// All implementations must embed UnimplementedGunServiceServer
// for forward compatibility.
type GunServiceServer interface {
	Tun(grpc.BidiStreamingServer[Hunk, Hunk]) error
	mustEmbedUnimplementedGunServiceServer()
}

// UnimplementedGunServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedGunServiceServer struct{}

func (UnimplementedGunServiceServer) Tun(grpc.BidiStreamingServer[Hunk, Hunk]) error {
	return status.Errorf(codes.Unimplemented, "method Tun not implemented")
}
func (UnimplementedGunServiceServer) mustEmbedUnimplementedGunServiceServer() {}
func (UnimplementedGunServiceServer) testEmbeddedByValue()                    {}

// UnsafeGunServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GunServiceServer will
// result in compilation errors.
type UnsafeGunServiceServer interface {
	mustEmbedUnimplementedGunServiceServer()
}

func RegisterGunServiceServer(s grpc.ServiceRegistrar, srv GunServiceServer) {
	// If the following call pancis, it indicates UnimplementedGunServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&GunService_ServiceDesc, srv)
}

func _GunService_Tun_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GunServiceServer).Tun(&grpc.GenericServerStream[Hunk, Hunk]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type GunService_TunServer = grpc.BidiStreamingServer[Hunk, Hunk]

// GunService_ServiceDesc is the grpc.ServiceDesc for GunService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var GunService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "transport.v2raygrpc.GunService",
	HandlerType: (*GunServiceServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "Tun",
			Handler:       _GunService_Tun_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "transport/v2raygrpc/stream.proto",
}
