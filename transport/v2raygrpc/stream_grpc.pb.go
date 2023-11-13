package v2raygrpc

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

const (
	GunService_Tun_FullMethodName = "/transport.v2raygrpc.GunService/Tun"
)

// GunServiceClient is the client API for GunService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type GunServiceClient interface {
	Tun(ctx context.Context, opts ...grpc.CallOption) (GunService_TunClient, error)
}

type gunServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewGunServiceClient(cc grpc.ClientConnInterface) GunServiceClient {
	return &gunServiceClient{cc}
}

func (c *gunServiceClient) Tun(ctx context.Context, opts ...grpc.CallOption) (GunService_TunClient, error) {
	stream, err := c.cc.NewStream(ctx, &GunService_ServiceDesc.Streams[0], GunService_Tun_FullMethodName, opts...)
	if err != nil {
		return nil, err
	}
	x := &gunServiceTunClient{stream}
	return x, nil
}

type GunService_TunClient interface {
	Send(*Hunk) error
	Recv() (*Hunk, error)
	grpc.ClientStream
}

type gunServiceTunClient struct {
	grpc.ClientStream
}

func (x *gunServiceTunClient) Send(m *Hunk) error {
	return x.ClientStream.SendMsg(m)
}

func (x *gunServiceTunClient) Recv() (*Hunk, error) {
	m := new(Hunk)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// GunServiceServer is the server API for GunService service.
// All implementations must embed UnimplementedGunServiceServer
// for forward compatibility
type GunServiceServer interface {
	Tun(GunService_TunServer) error
	mustEmbedUnimplementedGunServiceServer()
}

// UnimplementedGunServiceServer must be embedded to have forward compatible implementations.
type UnimplementedGunServiceServer struct{}

func (UnimplementedGunServiceServer) Tun(GunService_TunServer) error {
	return status.Errorf(codes.Unimplemented, "method Tun not implemented")
}
func (UnimplementedGunServiceServer) mustEmbedUnimplementedGunServiceServer() {}

// UnsafeGunServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to GunServiceServer will
// result in compilation errors.
type UnsafeGunServiceServer interface {
	mustEmbedUnimplementedGunServiceServer()
}

func RegisterGunServiceServer(s grpc.ServiceRegistrar, srv GunServiceServer) {
	s.RegisterService(&GunService_ServiceDesc, srv)
}

func _GunService_Tun_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(GunServiceServer).Tun(&gunServiceTunServer{stream})
}

type GunService_TunServer interface {
	Send(*Hunk) error
	Recv() (*Hunk, error)
	grpc.ServerStream
}

type gunServiceTunServer struct {
	grpc.ServerStream
}

func (x *gunServiceTunServer) Send(m *Hunk) error {
	return x.ServerStream.SendMsg(m)
}

func (x *gunServiceTunServer) Recv() (*Hunk, error) {
	m := new(Hunk)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

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
