package v2rayapi

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
	StatsService_GetStats_FullMethodName    = "/experimental.v2rayapi.StatsService/GetStats"
	StatsService_QueryStats_FullMethodName  = "/experimental.v2rayapi.StatsService/QueryStats"
	StatsService_GetSysStats_FullMethodName = "/experimental.v2rayapi.StatsService/GetSysStats"
)

// StatsServiceClient is the client API for StatsService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StatsServiceClient interface {
	GetStats(ctx context.Context, in *GetStatsRequest, opts ...grpc.CallOption) (*GetStatsResponse, error)
	QueryStats(ctx context.Context, in *QueryStatsRequest, opts ...grpc.CallOption) (*QueryStatsResponse, error)
	GetSysStats(ctx context.Context, in *SysStatsRequest, opts ...grpc.CallOption) (*SysStatsResponse, error)
}

type statsServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewStatsServiceClient(cc grpc.ClientConnInterface) StatsServiceClient {
	return &statsServiceClient{cc}
}

func (c *statsServiceClient) GetStats(ctx context.Context, in *GetStatsRequest, opts ...grpc.CallOption) (*GetStatsResponse, error) {
	out := new(GetStatsResponse)
	err := c.cc.Invoke(ctx, StatsService_GetStats_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *statsServiceClient) QueryStats(ctx context.Context, in *QueryStatsRequest, opts ...grpc.CallOption) (*QueryStatsResponse, error) {
	out := new(QueryStatsResponse)
	err := c.cc.Invoke(ctx, StatsService_QueryStats_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *statsServiceClient) GetSysStats(ctx context.Context, in *SysStatsRequest, opts ...grpc.CallOption) (*SysStatsResponse, error) {
	out := new(SysStatsResponse)
	err := c.cc.Invoke(ctx, StatsService_GetSysStats_FullMethodName, in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StatsServiceServer is the server API for StatsService service.
// All implementations must embed UnimplementedStatsServiceServer
// for forward compatibility
type StatsServiceServer interface {
	GetStats(context.Context, *GetStatsRequest) (*GetStatsResponse, error)
	QueryStats(context.Context, *QueryStatsRequest) (*QueryStatsResponse, error)
	GetSysStats(context.Context, *SysStatsRequest) (*SysStatsResponse, error)
	mustEmbedUnimplementedStatsServiceServer()
}

// UnimplementedStatsServiceServer must be embedded to have forward compatible implementations.
type UnimplementedStatsServiceServer struct{}

func (UnimplementedStatsServiceServer) GetStats(context.Context, *GetStatsRequest) (*GetStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetStats not implemented")
}

func (UnimplementedStatsServiceServer) QueryStats(context.Context, *QueryStatsRequest) (*QueryStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method QueryStats not implemented")
}

func (UnimplementedStatsServiceServer) GetSysStats(context.Context, *SysStatsRequest) (*SysStatsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSysStats not implemented")
}
func (UnimplementedStatsServiceServer) mustEmbedUnimplementedStatsServiceServer() {}

// UnsafeStatsServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StatsServiceServer will
// result in compilation errors.
type UnsafeStatsServiceServer interface {
	mustEmbedUnimplementedStatsServiceServer()
}

func RegisterStatsServiceServer(s grpc.ServiceRegistrar, srv StatsServiceServer) {
	s.RegisterService(&StatsService_ServiceDesc, srv)
}

func _StatsService_GetStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StatsServiceServer).GetStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StatsService_GetStats_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StatsServiceServer).GetStats(ctx, req.(*GetStatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StatsService_QueryStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(QueryStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StatsServiceServer).QueryStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StatsService_QueryStats_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StatsServiceServer).QueryStats(ctx, req.(*QueryStatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StatsService_GetSysStats_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SysStatsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StatsServiceServer).GetSysStats(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StatsService_GetSysStats_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StatsServiceServer).GetSysStats(ctx, req.(*SysStatsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// StatsService_ServiceDesc is the grpc.ServiceDesc for StatsService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var StatsService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "experimental.v2rayapi.StatsService",
	HandlerType: (*StatsServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetStats",
			Handler:    _StatsService_GetStats_Handler,
		},
		{
			MethodName: "QueryStats",
			Handler:    _StatsService_QueryStats_Handler,
		},
		{
			MethodName: "GetSysStats",
			Handler:    _StatsService_GetSysStats_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "experimental/v2rayapi/stats.proto",
}
