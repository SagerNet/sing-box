package daemon

import (
	"context"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing/common/memory"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var _ ManagedServiceServer = (*ManagedService)(nil)

type ManagedService struct {
	handler     ManagedHandler
	debug       bool
	oomReporter oomkiller.OOMReporter
}

type ManagedServiceOptions struct {
	Handler     ManagedHandler
	Debug       bool
	OOMReporter oomkiller.OOMReporter
}

func NewManagedService(options ManagedServiceOptions) *ManagedService {
	return &ManagedService{
		handler:     options.Handler,
		debug:       options.Debug,
		oomReporter: options.OOMReporter,
	}
}

func (s *ManagedService) StopService(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.handler.ServiceStop()
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ManagedService) ReloadService(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.handler.ServiceReload()
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ManagedService) GetSystemProxyStatus(ctx context.Context, empty *emptypb.Empty) (*SystemProxyStatus, error) {
	return s.handler.SystemProxyStatus()
}

func (s *ManagedService) SetSystemProxyEnabled(ctx context.Context, request *SetSystemProxyEnabledRequest) (*emptypb.Empty, error) {
	err := s.handler.SetSystemProxyEnabled(request.Enabled)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *ManagedService) TriggerDebugCrash(ctx context.Context, request *DebugCrashRequest) (*emptypb.Empty, error) {
	if !s.debug {
		return nil, status.Error(codes.PermissionDenied, "debug crash trigger unavailable")
	}
	if request == nil {
		return nil, status.Error(codes.InvalidArgument, "missing debug crash request")
	}
	switch request.Type {
	case DebugCrashRequest_GO:
		time.AfterFunc(200*time.Millisecond, func() {
			*(*int)(unsafe.Pointer(uintptr(0))) = 0
		})
	case DebugCrashRequest_NATIVE:
		err := s.handler.TriggerNativeCrash()
		if err != nil {
			return nil, err
		}
	default:
		return nil, status.Error(codes.InvalidArgument, "unknown debug crash type")
	}
	return &emptypb.Empty{}, nil
}

func (s *ManagedService) TriggerOOMReport(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	if s.oomReporter == nil {
		return nil, status.Error(codes.Unavailable, "OOM reporter not available")
	}
	return &emptypb.Empty{}, s.oomReporter.WriteReport(memory.Total())
}

func (s *ManagedService) mustEmbedUnimplementedManagedServiceServer() {
}
