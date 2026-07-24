package main

import (
	context "context"

	daemon "github.com/sagernet/sing-box/daemon"
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
	DesktopService_GetDaemonInfo_FullMethodName           = "/desktop.DesktopService/GetDaemonInfo"
	DesktopService_ClaimService_FullMethodName            = "/desktop.DesktopService/ClaimService"
	DesktopService_TakeOverService_FullMethodName         = "/desktop.DesktopService/TakeOverService"
	DesktopService_StartService_FullMethodName            = "/desktop.DesktopService/StartService"
	DesktopService_GetWorkingDirectory_FullMethodName     = "/desktop.DesktopService/GetWorkingDirectory"
	DesktopService_DestroyWorkingDirectory_FullMethodName = "/desktop.DesktopService/DestroyWorkingDirectory"
	DesktopService_ListCrashReports_FullMethodName        = "/desktop.DesktopService/ListCrashReports"
	DesktopService_ReadCrashReport_FullMethodName         = "/desktop.DesktopService/ReadCrashReport"
	DesktopService_MarkCrashReportRead_FullMethodName     = "/desktop.DesktopService/MarkCrashReportRead"
	DesktopService_ExportCrashReport_FullMethodName       = "/desktop.DesktopService/ExportCrashReport"
	DesktopService_DeleteCrashReport_FullMethodName       = "/desktop.DesktopService/DeleteCrashReport"
	DesktopService_DeleteAllCrashReports_FullMethodName   = "/desktop.DesktopService/DeleteAllCrashReports"
	DesktopService_ListOOMReports_FullMethodName          = "/desktop.DesktopService/ListOOMReports"
	DesktopService_ReadOOMReport_FullMethodName           = "/desktop.DesktopService/ReadOOMReport"
	DesktopService_MarkOOMReportRead_FullMethodName       = "/desktop.DesktopService/MarkOOMReportRead"
	DesktopService_ExportOOMReport_FullMethodName         = "/desktop.DesktopService/ExportOOMReport"
	DesktopService_DeleteOOMReport_FullMethodName         = "/desktop.DesktopService/DeleteOOMReport"
	DesktopService_DeleteAllOOMReports_FullMethodName     = "/desktop.DesktopService/DeleteAllOOMReports"
	DesktopService_InstallUpdate_FullMethodName           = "/desktop.DesktopService/InstallUpdate"
	DesktopService_GetSecuritySettings_FullMethodName     = "/desktop.DesktopService/GetSecuritySettings"
	DesktopService_SetInsecureModeEnabled_FullMethodName  = "/desktop.DesktopService/SetInsecureModeEnabled"
)

// DesktopServiceClient is the client API for DesktopService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type DesktopServiceClient interface {
	GetDaemonInfo(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*DaemonInfo, error)
	ClaimService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	TakeOverService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	StartService(ctx context.Context, in *StartServiceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	GetWorkingDirectory(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*WorkingDirectoryInfo, error)
	DestroyWorkingDirectory(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	ListCrashReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*CrashReportList, error)
	ReadCrashReport(ctx context.Context, in *CrashReportRequest, opts ...grpc.CallOption) (*CrashReportContent, error)
	MarkCrashReportRead(ctx context.Context, in *CrashReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	ExportCrashReport(ctx context.Context, in *CrashReportExportRequest, opts ...grpc.CallOption) (*CrashReportArchive, error)
	DeleteCrashReport(ctx context.Context, in *CrashReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	DeleteAllCrashReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	ListOOMReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*OOMReportList, error)
	ReadOOMReport(ctx context.Context, in *OOMReportRequest, opts ...grpc.CallOption) (*OOMReportContent, error)
	MarkOOMReportRead(ctx context.Context, in *OOMReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	ExportOOMReport(ctx context.Context, in *OOMReportExportRequest, opts ...grpc.CallOption) (*CrashReportArchive, error)
	DeleteOOMReport(ctx context.Context, in *OOMReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	DeleteAllOOMReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error)
	InstallUpdate(ctx context.Context, in *InstallUpdateRequest, opts ...grpc.CallOption) (*InstallUpdateResponse, error)
	GetSecuritySettings(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SecuritySettings, error)
	SetInsecureModeEnabled(ctx context.Context, in *SetInsecureModeEnabledRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}

type desktopServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewDesktopServiceClient(cc grpc.ClientConnInterface) DesktopServiceClient {
	return &desktopServiceClient{cc}
}

func (c *desktopServiceClient) GetDaemonInfo(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*DaemonInfo, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(DaemonInfo)
	err := c.cc.Invoke(ctx, DesktopService_GetDaemonInfo_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) ClaimService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_ClaimService_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) TakeOverService(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_TakeOverService_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) StartService(ctx context.Context, in *StartServiceRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_StartService_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) GetWorkingDirectory(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*WorkingDirectoryInfo, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(WorkingDirectoryInfo)
	err := c.cc.Invoke(ctx, DesktopService_GetWorkingDirectory_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) DestroyWorkingDirectory(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_DestroyWorkingDirectory_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) ListCrashReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*CrashReportList, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CrashReportList)
	err := c.cc.Invoke(ctx, DesktopService_ListCrashReports_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) ReadCrashReport(ctx context.Context, in *CrashReportRequest, opts ...grpc.CallOption) (*CrashReportContent, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CrashReportContent)
	err := c.cc.Invoke(ctx, DesktopService_ReadCrashReport_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) MarkCrashReportRead(ctx context.Context, in *CrashReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_MarkCrashReportRead_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) ExportCrashReport(ctx context.Context, in *CrashReportExportRequest, opts ...grpc.CallOption) (*CrashReportArchive, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CrashReportArchive)
	err := c.cc.Invoke(ctx, DesktopService_ExportCrashReport_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) DeleteCrashReport(ctx context.Context, in *CrashReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_DeleteCrashReport_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) DeleteAllCrashReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_DeleteAllCrashReports_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) ListOOMReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*OOMReportList, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(OOMReportList)
	err := c.cc.Invoke(ctx, DesktopService_ListOOMReports_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) ReadOOMReport(ctx context.Context, in *OOMReportRequest, opts ...grpc.CallOption) (*OOMReportContent, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(OOMReportContent)
	err := c.cc.Invoke(ctx, DesktopService_ReadOOMReport_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) MarkOOMReportRead(ctx context.Context, in *OOMReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_MarkOOMReportRead_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) ExportOOMReport(ctx context.Context, in *OOMReportExportRequest, opts ...grpc.CallOption) (*CrashReportArchive, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(CrashReportArchive)
	err := c.cc.Invoke(ctx, DesktopService_ExportOOMReport_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) DeleteOOMReport(ctx context.Context, in *OOMReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_DeleteOOMReport_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) DeleteAllOOMReports(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_DeleteAllOOMReports_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) InstallUpdate(ctx context.Context, in *InstallUpdateRequest, opts ...grpc.CallOption) (*InstallUpdateResponse, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(InstallUpdateResponse)
	err := c.cc.Invoke(ctx, DesktopService_InstallUpdate_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) GetSecuritySettings(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*SecuritySettings, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(SecuritySettings)
	err := c.cc.Invoke(ctx, DesktopService_GetSecuritySettings_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *desktopServiceClient) SetInsecureModeEnabled(ctx context.Context, in *SetInsecureModeEnabledRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, DesktopService_SetInsecureModeEnabled_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DesktopServiceServer is the server API for DesktopService service.
// All implementations must embed UnimplementedDesktopServiceServer
// for forward compatibility.
type DesktopServiceServer interface {
	GetDaemonInfo(context.Context, *emptypb.Empty) (*DaemonInfo, error)
	ClaimService(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	TakeOverService(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	StartService(context.Context, *StartServiceRequest) (*emptypb.Empty, error)
	GetWorkingDirectory(context.Context, *emptypb.Empty) (*WorkingDirectoryInfo, error)
	DestroyWorkingDirectory(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	ListCrashReports(context.Context, *emptypb.Empty) (*CrashReportList, error)
	ReadCrashReport(context.Context, *CrashReportRequest) (*CrashReportContent, error)
	MarkCrashReportRead(context.Context, *CrashReportRequest) (*emptypb.Empty, error)
	ExportCrashReport(context.Context, *CrashReportExportRequest) (*CrashReportArchive, error)
	DeleteCrashReport(context.Context, *CrashReportRequest) (*emptypb.Empty, error)
	DeleteAllCrashReports(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	ListOOMReports(context.Context, *emptypb.Empty) (*OOMReportList, error)
	ReadOOMReport(context.Context, *OOMReportRequest) (*OOMReportContent, error)
	MarkOOMReportRead(context.Context, *OOMReportRequest) (*emptypb.Empty, error)
	ExportOOMReport(context.Context, *OOMReportExportRequest) (*CrashReportArchive, error)
	DeleteOOMReport(context.Context, *OOMReportRequest) (*emptypb.Empty, error)
	DeleteAllOOMReports(context.Context, *emptypb.Empty) (*emptypb.Empty, error)
	InstallUpdate(context.Context, *InstallUpdateRequest) (*InstallUpdateResponse, error)
	GetSecuritySettings(context.Context, *emptypb.Empty) (*SecuritySettings, error)
	SetInsecureModeEnabled(context.Context, *SetInsecureModeEnabledRequest) (*emptypb.Empty, error)
	mustEmbedUnimplementedDesktopServiceServer()
}

// UnimplementedDesktopServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedDesktopServiceServer struct{}

func (UnimplementedDesktopServiceServer) GetDaemonInfo(context.Context, *emptypb.Empty) (*DaemonInfo, error) {
	return nil, status.Error(codes.Unimplemented, "method GetDaemonInfo not implemented")
}

func (UnimplementedDesktopServiceServer) ClaimService(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method ClaimService not implemented")
}

func (UnimplementedDesktopServiceServer) TakeOverService(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method TakeOverService not implemented")
}

func (UnimplementedDesktopServiceServer) StartService(context.Context, *StartServiceRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method StartService not implemented")
}

func (UnimplementedDesktopServiceServer) GetWorkingDirectory(context.Context, *emptypb.Empty) (*WorkingDirectoryInfo, error) {
	return nil, status.Error(codes.Unimplemented, "method GetWorkingDirectory not implemented")
}

func (UnimplementedDesktopServiceServer) DestroyWorkingDirectory(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method DestroyWorkingDirectory not implemented")
}

func (UnimplementedDesktopServiceServer) ListCrashReports(context.Context, *emptypb.Empty) (*CrashReportList, error) {
	return nil, status.Error(codes.Unimplemented, "method ListCrashReports not implemented")
}

func (UnimplementedDesktopServiceServer) ReadCrashReport(context.Context, *CrashReportRequest) (*CrashReportContent, error) {
	return nil, status.Error(codes.Unimplemented, "method ReadCrashReport not implemented")
}

func (UnimplementedDesktopServiceServer) MarkCrashReportRead(context.Context, *CrashReportRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method MarkCrashReportRead not implemented")
}

func (UnimplementedDesktopServiceServer) ExportCrashReport(context.Context, *CrashReportExportRequest) (*CrashReportArchive, error) {
	return nil, status.Error(codes.Unimplemented, "method ExportCrashReport not implemented")
}

func (UnimplementedDesktopServiceServer) DeleteCrashReport(context.Context, *CrashReportRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method DeleteCrashReport not implemented")
}

func (UnimplementedDesktopServiceServer) DeleteAllCrashReports(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method DeleteAllCrashReports not implemented")
}

func (UnimplementedDesktopServiceServer) ListOOMReports(context.Context, *emptypb.Empty) (*OOMReportList, error) {
	return nil, status.Error(codes.Unimplemented, "method ListOOMReports not implemented")
}

func (UnimplementedDesktopServiceServer) ReadOOMReport(context.Context, *OOMReportRequest) (*OOMReportContent, error) {
	return nil, status.Error(codes.Unimplemented, "method ReadOOMReport not implemented")
}

func (UnimplementedDesktopServiceServer) MarkOOMReportRead(context.Context, *OOMReportRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method MarkOOMReportRead not implemented")
}

func (UnimplementedDesktopServiceServer) ExportOOMReport(context.Context, *OOMReportExportRequest) (*CrashReportArchive, error) {
	return nil, status.Error(codes.Unimplemented, "method ExportOOMReport not implemented")
}

func (UnimplementedDesktopServiceServer) DeleteOOMReport(context.Context, *OOMReportRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method DeleteOOMReport not implemented")
}

func (UnimplementedDesktopServiceServer) DeleteAllOOMReports(context.Context, *emptypb.Empty) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method DeleteAllOOMReports not implemented")
}

func (UnimplementedDesktopServiceServer) InstallUpdate(context.Context, *InstallUpdateRequest) (*InstallUpdateResponse, error) {
	return nil, status.Error(codes.Unimplemented, "method InstallUpdate not implemented")
}

func (UnimplementedDesktopServiceServer) GetSecuritySettings(context.Context, *emptypb.Empty) (*SecuritySettings, error) {
	return nil, status.Error(codes.Unimplemented, "method GetSecuritySettings not implemented")
}

func (UnimplementedDesktopServiceServer) SetInsecureModeEnabled(context.Context, *SetInsecureModeEnabledRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method SetInsecureModeEnabled not implemented")
}
func (UnimplementedDesktopServiceServer) mustEmbedUnimplementedDesktopServiceServer() {}
func (UnimplementedDesktopServiceServer) testEmbeddedByValue()                        {}

// UnsafeDesktopServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to DesktopServiceServer will
// result in compilation errors.
type UnsafeDesktopServiceServer interface {
	mustEmbedUnimplementedDesktopServiceServer()
}

func RegisterDesktopServiceServer(s grpc.ServiceRegistrar, srv DesktopServiceServer) {
	// If the following call panics, it indicates UnimplementedDesktopServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&DesktopService_ServiceDesc, srv)
}

func _DesktopService_GetDaemonInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).GetDaemonInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_GetDaemonInfo_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).GetDaemonInfo(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_ClaimService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).ClaimService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_ClaimService_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).ClaimService(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_TakeOverService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).TakeOverService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_TakeOverService_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).TakeOverService(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_StartService_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(StartServiceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).StartService(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_StartService_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).StartService(ctx, req.(*StartServiceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_GetWorkingDirectory_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).GetWorkingDirectory(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_GetWorkingDirectory_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).GetWorkingDirectory(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_DestroyWorkingDirectory_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).DestroyWorkingDirectory(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_DestroyWorkingDirectory_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).DestroyWorkingDirectory(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_ListCrashReports_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).ListCrashReports(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_ListCrashReports_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).ListCrashReports(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_ReadCrashReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CrashReportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).ReadCrashReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_ReadCrashReport_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).ReadCrashReport(ctx, req.(*CrashReportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_MarkCrashReportRead_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CrashReportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).MarkCrashReportRead(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_MarkCrashReportRead_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).MarkCrashReportRead(ctx, req.(*CrashReportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_ExportCrashReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CrashReportExportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).ExportCrashReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_ExportCrashReport_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).ExportCrashReport(ctx, req.(*CrashReportExportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_DeleteCrashReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CrashReportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).DeleteCrashReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_DeleteCrashReport_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).DeleteCrashReport(ctx, req.(*CrashReportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_DeleteAllCrashReports_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).DeleteAllCrashReports(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_DeleteAllCrashReports_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).DeleteAllCrashReports(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_ListOOMReports_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).ListOOMReports(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_ListOOMReports_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).ListOOMReports(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_ReadOOMReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OOMReportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).ReadOOMReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_ReadOOMReport_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).ReadOOMReport(ctx, req.(*OOMReportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_MarkOOMReportRead_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OOMReportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).MarkOOMReportRead(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_MarkOOMReportRead_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).MarkOOMReportRead(ctx, req.(*OOMReportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_ExportOOMReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OOMReportExportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).ExportOOMReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_ExportOOMReport_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).ExportOOMReport(ctx, req.(*OOMReportExportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_DeleteOOMReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(OOMReportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).DeleteOOMReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_DeleteOOMReport_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).DeleteOOMReport(ctx, req.(*OOMReportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_DeleteAllOOMReports_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).DeleteAllOOMReports(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_DeleteAllOOMReports_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).DeleteAllOOMReports(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_InstallUpdate_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(InstallUpdateRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).InstallUpdate(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_InstallUpdate_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).InstallUpdate(ctx, req.(*InstallUpdateRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_GetSecuritySettings_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).GetSecuritySettings(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_GetSecuritySettings_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).GetSecuritySettings(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _DesktopService_SetInsecureModeEnabled_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetInsecureModeEnabledRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(DesktopServiceServer).SetInsecureModeEnabled(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: DesktopService_SetInsecureModeEnabled_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(DesktopServiceServer).SetInsecureModeEnabled(ctx, req.(*SetInsecureModeEnabledRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// DesktopService_ServiceDesc is the grpc.ServiceDesc for DesktopService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var DesktopService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "desktop.DesktopService",
	HandlerType: (*DesktopServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "GetDaemonInfo",
			Handler:    _DesktopService_GetDaemonInfo_Handler,
		},
		{
			MethodName: "ClaimService",
			Handler:    _DesktopService_ClaimService_Handler,
		},
		{
			MethodName: "TakeOverService",
			Handler:    _DesktopService_TakeOverService_Handler,
		},
		{
			MethodName: "StartService",
			Handler:    _DesktopService_StartService_Handler,
		},
		{
			MethodName: "GetWorkingDirectory",
			Handler:    _DesktopService_GetWorkingDirectory_Handler,
		},
		{
			MethodName: "DestroyWorkingDirectory",
			Handler:    _DesktopService_DestroyWorkingDirectory_Handler,
		},
		{
			MethodName: "ListCrashReports",
			Handler:    _DesktopService_ListCrashReports_Handler,
		},
		{
			MethodName: "ReadCrashReport",
			Handler:    _DesktopService_ReadCrashReport_Handler,
		},
		{
			MethodName: "MarkCrashReportRead",
			Handler:    _DesktopService_MarkCrashReportRead_Handler,
		},
		{
			MethodName: "ExportCrashReport",
			Handler:    _DesktopService_ExportCrashReport_Handler,
		},
		{
			MethodName: "DeleteCrashReport",
			Handler:    _DesktopService_DeleteCrashReport_Handler,
		},
		{
			MethodName: "DeleteAllCrashReports",
			Handler:    _DesktopService_DeleteAllCrashReports_Handler,
		},
		{
			MethodName: "ListOOMReports",
			Handler:    _DesktopService_ListOOMReports_Handler,
		},
		{
			MethodName: "ReadOOMReport",
			Handler:    _DesktopService_ReadOOMReport_Handler,
		},
		{
			MethodName: "MarkOOMReportRead",
			Handler:    _DesktopService_MarkOOMReportRead_Handler,
		},
		{
			MethodName: "ExportOOMReport",
			Handler:    _DesktopService_ExportOOMReport_Handler,
		},
		{
			MethodName: "DeleteOOMReport",
			Handler:    _DesktopService_DeleteOOMReport_Handler,
		},
		{
			MethodName: "DeleteAllOOMReports",
			Handler:    _DesktopService_DeleteAllOOMReports_Handler,
		},
		{
			MethodName: "InstallUpdate",
			Handler:    _DesktopService_InstallUpdate_Handler,
		},
		{
			MethodName: "GetSecuritySettings",
			Handler:    _DesktopService_GetSecuritySettings_Handler,
		},
		{
			MethodName: "SetInsecureModeEnabled",
			Handler:    _DesktopService_SetInsecureModeEnabled_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "experimental/boxdd/desktop_service.proto",
}

const (
	ApplicationService_CheckConfig_FullMethodName                       = "/desktop.ApplicationService/CheckConfig"
	ApplicationService_FormatConfig_FullMethodName                      = "/desktop.ApplicationService/FormatConfig"
	ApplicationService_GenerateConfigSchema_FullMethodName              = "/desktop.ApplicationService/GenerateConfigSchema"
	ApplicationService_EncodeProfile_FullMethodName                     = "/desktop.ApplicationService/EncodeProfile"
	ApplicationService_DecodeProfile_FullMethodName                     = "/desktop.ApplicationService/DecodeProfile"
	ApplicationService_ArchiveReport_FullMethodName                     = "/desktop.ApplicationService/ArchiveReport"
	ApplicationService_StartStandaloneNetworkQualityTest_FullMethodName = "/desktop.ApplicationService/StartStandaloneNetworkQualityTest"
	ApplicationService_StartStandaloneSTUNTest_FullMethodName           = "/desktop.ApplicationService/StartStandaloneSTUNTest"
)

// ApplicationServiceClient is the client API for ApplicationService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ApplicationServiceClient interface {
	CheckConfig(ctx context.Context, in *ConfigContent, opts ...grpc.CallOption) (*emptypb.Empty, error)
	FormatConfig(ctx context.Context, in *ConfigContent, opts ...grpc.CallOption) (*ConfigContent, error)
	GenerateConfigSchema(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ConfigContent, error)
	EncodeProfile(ctx context.Context, in *ProfileContent, opts ...grpc.CallOption) (*ProfileData, error)
	DecodeProfile(ctx context.Context, in *ProfileData, opts ...grpc.CallOption) (*ProfileContent, error)
	ArchiveReport(ctx context.Context, in *ArchiveReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
	StartStandaloneNetworkQualityTest(ctx context.Context, in *StandaloneNetworkQualityTestRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemon.NetworkQualityTestProgress], error)
	StartStandaloneSTUNTest(ctx context.Context, in *StandaloneSTUNTestRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemon.STUNTestProgress], error)
}

type applicationServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewApplicationServiceClient(cc grpc.ClientConnInterface) ApplicationServiceClient {
	return &applicationServiceClient{cc}
}

func (c *applicationServiceClient) CheckConfig(ctx context.Context, in *ConfigContent, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, ApplicationService_CheckConfig_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applicationServiceClient) FormatConfig(ctx context.Context, in *ConfigContent, opts ...grpc.CallOption) (*ConfigContent, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ConfigContent)
	err := c.cc.Invoke(ctx, ApplicationService_FormatConfig_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applicationServiceClient) GenerateConfigSchema(ctx context.Context, in *emptypb.Empty, opts ...grpc.CallOption) (*ConfigContent, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ConfigContent)
	err := c.cc.Invoke(ctx, ApplicationService_GenerateConfigSchema_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applicationServiceClient) EncodeProfile(ctx context.Context, in *ProfileContent, opts ...grpc.CallOption) (*ProfileData, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ProfileData)
	err := c.cc.Invoke(ctx, ApplicationService_EncodeProfile_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applicationServiceClient) DecodeProfile(ctx context.Context, in *ProfileData, opts ...grpc.CallOption) (*ProfileContent, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(ProfileContent)
	err := c.cc.Invoke(ctx, ApplicationService_DecodeProfile_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applicationServiceClient) ArchiveReport(ctx context.Context, in *ArchiveReportRequest, opts ...grpc.CallOption) (*emptypb.Empty, error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	out := new(emptypb.Empty)
	err := c.cc.Invoke(ctx, ApplicationService_ArchiveReport_FullMethodName, in, out, cOpts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *applicationServiceClient) StartStandaloneNetworkQualityTest(ctx context.Context, in *StandaloneNetworkQualityTestRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemon.NetworkQualityTestProgress], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &ApplicationService_ServiceDesc.Streams[0], ApplicationService_StartStandaloneNetworkQualityTest_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[StandaloneNetworkQualityTestRequest, daemon.NetworkQualityTestProgress]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type ApplicationService_StartStandaloneNetworkQualityTestClient = grpc.ServerStreamingClient[daemon.NetworkQualityTestProgress]

func (c *applicationServiceClient) StartStandaloneSTUNTest(ctx context.Context, in *StandaloneSTUNTestRequest, opts ...grpc.CallOption) (grpc.ServerStreamingClient[daemon.STUNTestProgress], error) {
	cOpts := append([]grpc.CallOption{grpc.StaticMethod()}, opts...)
	stream, err := c.cc.NewStream(ctx, &ApplicationService_ServiceDesc.Streams[1], ApplicationService_StartStandaloneSTUNTest_FullMethodName, cOpts...)
	if err != nil {
		return nil, err
	}
	x := &grpc.GenericClientStream[StandaloneSTUNTestRequest, daemon.STUNTestProgress]{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type ApplicationService_StartStandaloneSTUNTestClient = grpc.ServerStreamingClient[daemon.STUNTestProgress]

// ApplicationServiceServer is the server API for ApplicationService service.
// All implementations must embed UnimplementedApplicationServiceServer
// for forward compatibility.
type ApplicationServiceServer interface {
	CheckConfig(context.Context, *ConfigContent) (*emptypb.Empty, error)
	FormatConfig(context.Context, *ConfigContent) (*ConfigContent, error)
	GenerateConfigSchema(context.Context, *emptypb.Empty) (*ConfigContent, error)
	EncodeProfile(context.Context, *ProfileContent) (*ProfileData, error)
	DecodeProfile(context.Context, *ProfileData) (*ProfileContent, error)
	ArchiveReport(context.Context, *ArchiveReportRequest) (*emptypb.Empty, error)
	StartStandaloneNetworkQualityTest(*StandaloneNetworkQualityTestRequest, grpc.ServerStreamingServer[daemon.NetworkQualityTestProgress]) error
	StartStandaloneSTUNTest(*StandaloneSTUNTestRequest, grpc.ServerStreamingServer[daemon.STUNTestProgress]) error
	mustEmbedUnimplementedApplicationServiceServer()
}

// UnimplementedApplicationServiceServer must be embedded to have
// forward compatible implementations.
//
// NOTE: this should be embedded by value instead of pointer to avoid a nil
// pointer dereference when methods are called.
type UnimplementedApplicationServiceServer struct{}

func (UnimplementedApplicationServiceServer) CheckConfig(context.Context, *ConfigContent) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method CheckConfig not implemented")
}

func (UnimplementedApplicationServiceServer) FormatConfig(context.Context, *ConfigContent) (*ConfigContent, error) {
	return nil, status.Error(codes.Unimplemented, "method FormatConfig not implemented")
}

func (UnimplementedApplicationServiceServer) GenerateConfigSchema(context.Context, *emptypb.Empty) (*ConfigContent, error) {
	return nil, status.Error(codes.Unimplemented, "method GenerateConfigSchema not implemented")
}

func (UnimplementedApplicationServiceServer) EncodeProfile(context.Context, *ProfileContent) (*ProfileData, error) {
	return nil, status.Error(codes.Unimplemented, "method EncodeProfile not implemented")
}

func (UnimplementedApplicationServiceServer) DecodeProfile(context.Context, *ProfileData) (*ProfileContent, error) {
	return nil, status.Error(codes.Unimplemented, "method DecodeProfile not implemented")
}

func (UnimplementedApplicationServiceServer) ArchiveReport(context.Context, *ArchiveReportRequest) (*emptypb.Empty, error) {
	return nil, status.Error(codes.Unimplemented, "method ArchiveReport not implemented")
}

func (UnimplementedApplicationServiceServer) StartStandaloneNetworkQualityTest(*StandaloneNetworkQualityTestRequest, grpc.ServerStreamingServer[daemon.NetworkQualityTestProgress]) error {
	return status.Error(codes.Unimplemented, "method StartStandaloneNetworkQualityTest not implemented")
}

func (UnimplementedApplicationServiceServer) StartStandaloneSTUNTest(*StandaloneSTUNTestRequest, grpc.ServerStreamingServer[daemon.STUNTestProgress]) error {
	return status.Error(codes.Unimplemented, "method StartStandaloneSTUNTest not implemented")
}
func (UnimplementedApplicationServiceServer) mustEmbedUnimplementedApplicationServiceServer() {}
func (UnimplementedApplicationServiceServer) testEmbeddedByValue()                            {}

// UnsafeApplicationServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ApplicationServiceServer will
// result in compilation errors.
type UnsafeApplicationServiceServer interface {
	mustEmbedUnimplementedApplicationServiceServer()
}

func RegisterApplicationServiceServer(s grpc.ServiceRegistrar, srv ApplicationServiceServer) {
	// If the following call panics, it indicates UnimplementedApplicationServiceServer was
	// embedded by pointer and is nil.  This will cause panics if an
	// unimplemented method is ever invoked, so we test this at initialization
	// time to prevent it from happening at runtime later due to I/O.
	if t, ok := srv.(interface{ testEmbeddedByValue() }); ok {
		t.testEmbeddedByValue()
	}
	s.RegisterService(&ApplicationService_ServiceDesc, srv)
}

func _ApplicationService_CheckConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigContent)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplicationServiceServer).CheckConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ApplicationService_CheckConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplicationServiceServer).CheckConfig(ctx, req.(*ConfigContent))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplicationService_FormatConfig_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ConfigContent)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplicationServiceServer).FormatConfig(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ApplicationService_FormatConfig_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplicationServiceServer).FormatConfig(ctx, req.(*ConfigContent))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplicationService_GenerateConfigSchema_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(emptypb.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplicationServiceServer).GenerateConfigSchema(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ApplicationService_GenerateConfigSchema_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplicationServiceServer).GenerateConfigSchema(ctx, req.(*emptypb.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplicationService_EncodeProfile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProfileContent)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplicationServiceServer).EncodeProfile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ApplicationService_EncodeProfile_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplicationServiceServer).EncodeProfile(ctx, req.(*ProfileContent))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplicationService_DecodeProfile_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ProfileData)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplicationServiceServer).DecodeProfile(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ApplicationService_DecodeProfile_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplicationServiceServer).DecodeProfile(ctx, req.(*ProfileData))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplicationService_ArchiveReport_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ArchiveReportRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ApplicationServiceServer).ArchiveReport(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: ApplicationService_ArchiveReport_FullMethodName,
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ApplicationServiceServer).ArchiveReport(ctx, req.(*ArchiveReportRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ApplicationService_StartStandaloneNetworkQualityTest_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StandaloneNetworkQualityTestRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ApplicationServiceServer).StartStandaloneNetworkQualityTest(m, &grpc.GenericServerStream[StandaloneNetworkQualityTestRequest, daemon.NetworkQualityTestProgress]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type ApplicationService_StartStandaloneNetworkQualityTestServer = grpc.ServerStreamingServer[daemon.NetworkQualityTestProgress]

func _ApplicationService_StartStandaloneSTUNTest_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(StandaloneSTUNTestRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ApplicationServiceServer).StartStandaloneSTUNTest(m, &grpc.GenericServerStream[StandaloneSTUNTestRequest, daemon.STUNTestProgress]{ServerStream: stream})
}

// This type alias is provided for backwards compatibility with existing code that references the prior non-generic stream type by name.
type ApplicationService_StartStandaloneSTUNTestServer = grpc.ServerStreamingServer[daemon.STUNTestProgress]

// ApplicationService_ServiceDesc is the grpc.ServiceDesc for ApplicationService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ApplicationService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "desktop.ApplicationService",
	HandlerType: (*ApplicationServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CheckConfig",
			Handler:    _ApplicationService_CheckConfig_Handler,
		},
		{
			MethodName: "FormatConfig",
			Handler:    _ApplicationService_FormatConfig_Handler,
		},
		{
			MethodName: "GenerateConfigSchema",
			Handler:    _ApplicationService_GenerateConfigSchema_Handler,
		},
		{
			MethodName: "EncodeProfile",
			Handler:    _ApplicationService_EncodeProfile_Handler,
		},
		{
			MethodName: "DecodeProfile",
			Handler:    _ApplicationService_DecodeProfile_Handler,
		},
		{
			MethodName: "ArchiveReport",
			Handler:    _ApplicationService_ArchiveReport_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "StartStandaloneNetworkQualityTest",
			Handler:       _ApplicationService_StartStandaloneNetworkQualityTest_Handler,
			ServerStreams: true,
		},
		{
			StreamName:    "StartStandaloneSTUNTest",
			Handler:       _ApplicationService_StartStandaloneSTUNTest_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "experimental/boxdd/desktop_service.proto",
}
