package daemon

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.64.0 or later.
const _ = grpc.SupportPackageIsVersion9

const (
	StartedService_StopService_FullMethodName            = "/daemon.StartedService/StopService"
	StartedService_ReloadService_FullMethodName          = "/daemon.StartedService/ReloadService"
	StartedService_SubscribeServiceStatus_FullMethodName = "/daemon.StartedService/SubscribeServiceStatus"
	StartedService_SubscribeLog_FullMethodName           = "/daemon.StartedService/SubscribeLog"
	StartedService_GetDefaultLogLevel_FullMethodName     = "/daemon.StartedService/GetDefaultLogLevel"
	StartedService_SubscribeStatus_FullMethodName        = "/daemon.StartedService/SubscribeStatus"
	StartedService_SubscribeGroups_FullMethodName        = "/daemon.StartedService/SubscribeGroups"
	StartedService_GetClashModeStatus_FullMethodName     = "/daemon.StartedService/GetClashModeStatus"
	StartedService_SubscribeClashMode_FullMethodName     = "/daemon.StartedService/SubscribeClashMode"
	StartedService_SetClashMode_FullMethodName           = "/daemon.StartedService/SetClashMode"
	StartedService_URLTest_FullMethodName                = "/daemon.StartedService/URLTest"
	StartedService_SelectOutbound_FullMethodName         = "/daemon.StartedService/SelectOutbound"
	StartedService_SetGroupExpand_FullMethodName         = "/daemon.StartedService/SetGroupExpand"
	StartedService_GetSystemProxyStatus_FullMethodName   = "/daemon.StartedService/GetSystemProxyStatus"
	StartedService_SetSystemProxyEnabled_FullMethodName  = "/daemon.StartedService/SetSystemProxyEnabled"
	StartedService_SubscribeConnections_FullMethodName   = "/daemon.StartedService/SubscribeConnections"
	StartedService_CloseConnection_FullMethodName        = "/daemon.StartedService/CloseConnection"
	StartedService_CloseAllConnections_FullMethodName    = "/daemon.StartedService/CloseAllConnections"
	StartedService_GetDeprecatedWarnings_FullMethodName  = "/daemon.StartedService/GetDeprecatedWarnings"
	StartedService_SubscribeHelperEvents_FullMethodName  = "/daemon.StartedService/SubscribeHelperEvents"
	StartedService_SendHelperResponse_FullMethodName     = "/daemon.StartedService/SendHelperResponse"
)

// StartedServiceClient is the client API for StartedService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type StartedServiceClient interface {
	StopService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	ReloadService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	SubscribeServiceStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ServiceStatus], error)
	SubscribeLog(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Log], error)
	GetDefaultLogLevel(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*DefaultLogLevel, error)
	SubscribeStatus(ctx context.Context, in *SubscribeStatusRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Status], error)
	SubscribeGroups(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Groups], error)
	GetClashModeStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ClashModeStatus, error)
	SubscribeClashMode(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ClashMode], error)
	SetClashMode(ctx context.Context, in *ClashMode, opts ...grpc.CallOption) (*emptypb.Empty, error)
	URLTest(ctx context.Context, in *URLTestRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	SelectOutbound(ctx context.Context, in *SelectOutboundRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	SetGroupExpand(ctx context.Context, in *SetGroupExpandRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	GetSystemProxyStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SystemProxyStatus, error)
	SetSystemProxyEnabled(ctx context.Context, in *SetSystemProxyEnabledRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	SubscribeConnections(ctx context.Context, in *SubscribeConnectionsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Connections], error)
	CloseConnection(ctx context.Context, in *CloseConnectionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	CloseAllConnections(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	GetDeprecatedWarnings(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*DeprecatedWarnings, error)
	SubscribeHelperEvents(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[HelperRequest], error)
	SendHelperResponse(ctx context.Context, in *HelperResponse, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type startedServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewStartedServiceClient(cc grpc.ClientConnInterface) StartedServiceClient {
	return &startedServiceClient{cc}
}

func (c *startedServiceClient) StopService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_StopService_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) ReloadService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_ReloadService_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SubscribeServiceStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ServiceStatus], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &StartedService_ServiceDesc.Streams[0], StartedService_SubscribeServiceStatus_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[emptypb.Empty, ServiceStatus]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeServiceStatusClient = grpc.ServerStreamingClient[ServiceStatus]

func (c *startedServiceClient) SubscribeLog(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Log], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &StartedService_ServiceDesc.Streams[1], StartedService_SubscribeLog_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[emptypb.Empty, Log]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeLogClient = grpc.ServerStreamingClient[Log]

func (c *startedServiceClient) GetDefaultLogLevel(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*DefaultLogLevel, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DefaultLogLevel)
	err := c.cc.Invoke(ctx, StartedService_GetDefaultLogLevel_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SubscribeStatus(ctx context.Context, in *SubscribeStatusRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Status], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &StartedService_ServiceDesc.Streams[2], StartedService_SubscribeStatus_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[SubscribeStatusRequest, Status]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeStatusClient = grpc.ServerStreamingClient[Status]

func (c *startedServiceClient) SubscribeGroups(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Groups], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &StartedService_ServiceDesc.Streams[3], StartedService_SubscribeGroups_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[emptypb.Empty, Groups]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeGroupsClient = grpc.ServerStreamingClient[Groups]

func (c *startedServiceClient) GetClashModeStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ClashModeStatus, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ClashModeStatus)
	err := c.cc.Invoke(ctx, StartedService_GetClashModeStatus_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SubscribeClashMode(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[ClashMode], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &StartedService_ServiceDesc.Streams[4], StartedService_SubscribeClashMode_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[emptypb.Empty, ClashMode]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeClashModeClient = grpc.ServerStreamingClient[ClashMode]

func (c *startedServiceClient) SetClashMode(ctx context.Context, in *ClashMode, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_SetClashMode_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) URLTest(ctx context.Context, in *URLTestRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_URLTest_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SelectOutbound(ctx context.Context, in *SelectOutboundRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_SelectOutbound_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SetGroupExpand(ctx context.Context, in *SetGroupExpandRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_SetGroupExpand_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) GetSystemProxyStatus(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SystemProxyStatus, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SystemProxyStatus)
	err := c.cc.Invoke(ctx, StartedService_GetSystemProxyStatus_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SetSystemProxyEnabled(ctx context.Context, in *SetSystemProxyEnabledRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_SetSystemProxyEnabled_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SubscribeConnections(ctx context.Context, in *SubscribeConnectionsRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[Connections], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &StartedService_ServiceDesc.Streams[5], StartedService_SubscribeConnections_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[SubscribeConnectionsRequest, Connections]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeConnectionsClient = grpc.ServerStreamingClient[Connections]

func (c *startedServiceClient) CloseConnection(ctx context.Context, in *CloseConnectionRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_CloseConnection_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) CloseAllConnections(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_CloseAllConnections_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) GetDeprecatedWarnings(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*DeprecatedWarnings, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DeprecatedWarnings)
	err := c.cc.Invoke(ctx, StartedService_GetDeprecatedWarnings_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *startedServiceClient) SubscribeHelperEvents(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (grpc.ServerStreamingClient[HelperRequest], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &StartedService_ServiceDesc.Streams[6], StartedService_SubscribeHelperEvents_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[emptypb.Empty, HelperRequest]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeHelperEventsClient = grpc.ServerStreamingClient[HelperRequest]

func (c *startedServiceClient) SendHelperResponse(ctx context.Context, in *HelperResponse, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, StartedService_SendHelperResponse_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// StartedServiceServer is the server API for StartedService service.
// All implementations must embed UnimplementedStartedServiceServer
// for forward compatibility.
type StartedServiceServer interface {
	StopService(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	ReloadService(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	SubscribeServiceStatus(*emptypb.Empty, grpc.ServerStreamingServer[ServiceStatus]) error
	SubscribeLog(*emptypb.Empty, grpc.ServerStreamingServer[Log]) error
	GetDefaultLogLevel(context.Context, *emptypb.Empty) (*DefaultLogLevel, error)
	SubscribeStatus(*SubscribeStatusRequest, grpc.ServerStreamingServer[Status]) error
	SubscribeGroups(*emptypb.Empty, grpc.ServerStreamingServer[Groups]) error
	GetClashModeStatus(context.Context, *emptypb.Empty) (*ClashModeStatus, error)
	SubscribeClashMode(*emptypb.Empty, grpc.ServerStreamingServer[ClashMode]) error
	SetClashMode(context.Context, *ClashMode) (*emptypb.Empty, error)
	URLTest(context.Context, *URLTestRequest) (*emptypb.Empty, error)
	SelectOutbound(context.Context, *SelectOutboundRequest) (*emptypb.Empty, error)
	SetGroupExpand(context.Context, *SetGroupExpandRequest) (*emptypb.Empty, error)
	GetSystemProxyStatus(context.Context, *emptypb.Empty) (*SystemProxyStatus, error)
	SetSystemProxyEnabled(context.Context, *SetSystemProxyEnabledRequest) (*emptypb.Empty, error)
	SubscribeConnections(*SubscribeConnectionsRequest, grpc.ServerStreamingServer[Connections]) error
	CloseConnection(context.Context, *CloseConnectionRequest) (*emptypb.Empty, error)
	CloseAllConnections(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	GetDeprecatedWarnings(context.Context, *emptypb.Empty) (*DeprecatedWarnings, error)
	SubscribeHelperEvents(*emptypb.Empty, grpc.ServerStreamingServer[HelperRequest]) error
	SendHelperResponse(context.Context, *HelperResponse) (*emptypb.Empty, error)
	mustEmbedUnimplementedStartedServiceServer()
}

// UnimplementedStartedServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedStartedServiceServer struct{}

func (UnimplementedStartedServiceServer) StopService(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method StopService not implemented")
}

func (UnimplementedStartedServiceServer) ReloadService(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReloadService not implemented")
}

func (UnimplementedStartedServiceServer) SubscribeServiceStatus(*emptypb.Empty, grpc.ServerStreamingServer[ServiceStatus]) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeServiceStatus not implemented")
}

func (UnimplementedStartedServiceServer) SubscribeLog(*emptypb.Empty, grpc.ServerStreamingServer[Log]) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeLog not implemented")
}

func (UnimplementedStartedServiceServer) GetDefaultLogLevel(context.Context, *emptypb.Empty) (*DefaultLogLevel, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDefaultLogLevel not implemented")
}

func (UnimplementedStartedServiceServer) SubscribeStatus(*SubscribeStatusRequest, grpc.ServerStreamingServer[Status]) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeStatus not implemented")
}

func (UnimplementedStartedServiceServer) SubscribeGroups(*emptypb.Empty, grpc.ServerStreamingServer[Groups]) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeGroups not implemented")
}

func (UnimplementedStartedServiceServer) GetClashModeStatus(context.Context, *emptypb.Empty) (*ClashModeStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetClashModeStatus not implemented")
}

func (UnimplementedStartedServiceServer) SubscribeClashMode(*emptypb.Empty, grpc.ServerStreamingServer[ClashMode]) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeClashMode not implemented")
}

func (UnimplementedStartedServiceServer) SetClashMode(context.Context, *ClashMode) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetClashMode not implemented")
}

func (UnimplementedStartedServiceServer) URLTest(context.Context, *URLTestRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method URLTest not implemented")
}

func (UnimplementedStartedServiceServer) SelectOutbound(context.Context, *SelectOutboundRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SelectOutbound not implemented")
}

func (UnimplementedStartedServiceServer) SetGroupExpand(context.Context, *SetGroupExpandRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetGroupExpand not implemented")
}

func (UnimplementedStartedServiceServer) GetSystemProxyStatus(context.Context, *emptypb.Empty) (*SystemProxyStatus, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetSystemProxyStatus not implemented")
}

func (UnimplementedStartedServiceServer) SetSystemProxyEnabled(context.Context, *SetSystemProxyEnabledRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetSystemProxyEnabled not implemented")
}

func (UnimplementedStartedServiceServer) SubscribeConnections(*SubscribeConnectionsRequest, grpc.ServerStreamingServer[Connections]) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeConnections not implemented")
}

func (UnimplementedStartedServiceServer) CloseConnection(context.Context, *CloseConnectionRequest) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseConnection not implemented")
}

func (UnimplementedStartedServiceServer) CloseAllConnections(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method CloseAllConnections not implemented")
}

func (UnimplementedStartedServiceServer) GetDeprecatedWarnings(context.Context, *emptypb.Empty) (*DeprecatedWarnings, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetDeprecatedWarnings not implemented")
}

func (UnimplementedStartedServiceServer) SubscribeHelperEvents(*emptypb.Empty, grpc.ServerStreamingServer[HelperRequest]) error {
	return status.Errorf(codes.Unimplemented, "method SubscribeHelperEvents not implemented")
}

func (UnimplementedStartedServiceServer) SendHelperResponse(context.Context, *HelperResponse) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SendHelperResponse not implemented")
}
func (UnimplementedStartedServiceServer) mustEmbedUnimplementedStartedServiceServer() {}
func (UnimplementedStartedServiceServer) testEmbeddedByValue()                        {}

// UnsafeStartedServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to StartedServiceServer will
// result in compilation errors.
type UnsafeStartedServiceServer interface {
	mustEmbedUnimplementedStartedServiceServer()
}

func RegisterStartedServiceServer(s grpc.ServiceRegistrar, srv StartedServiceServer) {
	// If the following call pancis, it indicates UnimplementedStartedServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&StartedService_ServiceDesc, srv)
}

func _StartedService_StopService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).StopService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_StopService_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).StopService(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_ReloadService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).ReloadService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_ReloadService_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).ReloadService(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SubscribeServiceStatus_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(emptypb.Empty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StartedServiceServer).SubscribeServiceStatus(m, &grpc.GenericServerStream[emptypb.Empty, ServiceStatus]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeServiceStatusServer = grpc.ServerStreamingServer[ServiceStatus]

func _StartedService_SubscribeLog_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(emptypb.Empty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StartedServiceServer).SubscribeLog(m, &grpc.GenericServerStream[emptypb.Empty, Log]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeLogServer = grpc.ServerStreamingServer[Log]

func _StartedService_GetDefaultLogLevel_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).GetDefaultLogLevel(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_GetDefaultLogLevel_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).GetDefaultLogLevel(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SubscribeStatus_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeStatusRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StartedServiceServer).SubscribeStatus(m, &grpc.GenericServerStream[SubscribeStatusRequest, Status]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeStatusServer = grpc.ServerStreamingServer[Status]

func _StartedService_SubscribeGroups_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(emptypb.Empty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StartedServiceServer).SubscribeGroups(m, &grpc.GenericServerStream[emptypb.Empty, Groups]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeGroupsServer = grpc.ServerStreamingServer[Groups]

func _StartedService_GetClashModeStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).GetClashModeStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_GetClashModeStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).GetClashModeStatus(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SubscribeClashMode_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(emptypb.Empty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StartedServiceServer).SubscribeClashMode(m, &grpc.GenericServerStream[emptypb.Empty, ClashMode]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeClashModeServer = grpc.ServerStreamingServer[ClashMode]

func _StartedService_SetClashMode_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ClashMode)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).SetClashMode(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_SetClashMode_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).SetClashMode(ctx, req.(*ClashMode))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_URLTest_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(URLTestRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).URLTest(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_URLTest_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).URLTest(ctx, req.(*URLTestRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SelectOutbound_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SelectOutboundRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).SelectOutbound(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_SelectOutbound_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).SelectOutbound(ctx, req.(*SelectOutboundRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SetGroupExpand_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetGroupExpandRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).SetGroupExpand(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_SetGroupExpand_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).SetGroupExpand(ctx, req.(*SetGroupExpandRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_GetSystemProxyStatus_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).GetSystemProxyStatus(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_GetSystemProxyStatus_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).GetSystemProxyStatus(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SetSystemProxyEnabled_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetSystemProxyEnabledRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).SetSystemProxyEnabled(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_SetSystemProxyEnabled_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).SetSystemProxyEnabled(ctx, req.(*SetSystemProxyEnabledRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SubscribeConnections_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(SubscribeConnectionsRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StartedServiceServer).SubscribeConnections(m, &grpc.GenericServerStream[SubscribeConnectionsRequest, Connections]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeConnectionsServer = grpc.ServerStreamingServer[Connections]

func _StartedService_CloseConnection_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CloseConnectionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).CloseConnection(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_CloseConnection_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).CloseConnection(ctx, req.(*CloseConnectionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_CloseAllConnections_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).CloseAllConnections(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_CloseAllConnections_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).CloseAllConnections(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_GetDeprecatedWarnings_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).GetDeprecatedWarnings(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_GetDeprecatedWarnings_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).GetDeprecatedWarnings(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _StartedService_SubscribeHelperEvents_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(emptypb.Empty)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(StartedServiceServer).SubscribeHelperEvents(m, &grpc.GenericServerStream[emptypb.Empty, HelperRequest]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type StartedService_SubscribeHelperEventsServer = grpc.ServerStreamingServer[HelperRequest]

func _StartedService_SendHelperResponse_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(HelperResponse)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(StartedServiceServer).SendHelperResponse(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: StartedService_SendHelperResponse_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(StartedServiceServer).SendHelperResponse(ctx, req.(*HelperResponse))
	}
	return interceptor(ctx, in, info, handler)
}

// StartedService_ServiceDesc is the grpc.ServiceDesc for StartedService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var StartedService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "daemon.StartedService",
	HandlerType: (*StartedServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "StopService",
			Handler:    _StartedService_StopService_Handler,
		},
		{
			MethodName: "ReloadService",
			Handler:    _StartedService_ReloadService_Handler,
		},
		{
			MethodName: "GetDefaultLogLevel",
			Handler:    _StartedService_GetDefaultLogLevel_Handler,
		},
		{
			MethodName: "GetClashModeStatus",
			Handler:    _StartedService_GetClashModeStatus_Handler,
		},
		{
			MethodName: "SetClashMode",
			Handler:    _StartedService_SetClashMode_Handler,
		},
		{
			MethodName: "URLTest",
			Handler:    _StartedService_URLTest_Handler,
		},
		{
			MethodName: "SelectOutbound",
			Handler:    _StartedService_SelectOutbound_Handler,
		},
		{
			MethodName: "SetGroupExpand",
			Handler:    _StartedService_SetGroupExpand_Handler,
		},
		{
			MethodName: "GetSystemProxyStatus",
			Handler:    _StartedService_GetSystemProxyStatus_Handler,
		},
		{
			MethodName: "SetSystemProxyEnabled",
			Handler:    _StartedService_SetSystemProxyEnabled_Handler,
		},
		{
			MethodName: "CloseConnection",
			Handler:    _StartedService_CloseConnection_Handler,
		},
		{
			MethodName: "CloseAllConnections",
			Handler:    _StartedService_CloseAllConnections_Handler,
		},
		{
			MethodName: "GetDeprecatedWarnings",
			Handler:    _StartedService_GetDeprecatedWarnings_Handler,
		},
		{
			MethodName: "SendHelperResponse",
			Handler:    _StartedService_SendHelperResponse_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "SubscribeServiceStatus",
			Handler:       _StartedService_SubscribeServiceStatus_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "SubscribeLog",
			Handler:       _StartedService_SubscribeLog_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "SubscribeStatus",
			Handler:       _StartedService_SubscribeStatus_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "SubscribeGroups",
			Handler:       _StartedService_SubscribeGroups_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "SubscribeClashMode",
			Handler:       _StartedService_SubscribeClashMode_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "SubscribeConnections",
			Handler:       _StartedService_SubscribeConnections_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "SubscribeHelperEvents",
			Handler:       _StartedService_SubscribeHelperEvents_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "daemon/started_service.proto",
}
