package daemon

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"
	"unsafe"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/networkquality"
	"github.com/sagernet/sing-box/common/stun"
	"github.com/sagernet/sing-box/common/urltest"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing-box/service/oomkiller"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/gofrs/uuid/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var _ StartedServiceServer = (*StartedService)(nil)

type StartedService struct {
	ctx context.Context
	// platform adapter.PlatformInterface
	handler           PlatformHandler
	debug             bool
	logMaxLines       int
	oomKillerEnabled  bool
	oomKillerDisabled bool
	oomMemoryLimit    uint64
	// workingDirectory string
	// tempDirectory    string
	// userID           int
	// groupID          int
	// systemProxyEnabled      bool
	serviceAccess           sync.RWMutex
	serviceStatus           *ServiceStatus
	serviceStatusSubscriber *observable.Subscriber[*ServiceStatus]
	serviceStatusObserver   *observable.Observer[*ServiceStatus]
	logAccess               sync.RWMutex
	logLines                list.List[*log.Entry]
	logSubscriber           *observable.Subscriber[*log.Entry]
	logObserver             *observable.Observer[*log.Entry]
	instance                *Instance
	startedAt               time.Time
	urlTestSubscriber       *observable.Subscriber[struct{}]
	urlTestObserver         *observable.Observer[struct{}]
	urlTestHistoryStorage   *urltest.HistoryStorage
	clashModeSubscriber     *observable.Subscriber[struct{}]
	clashModeObserver       *observable.Observer[struct{}]

	connectionEventSubscriber *observable.Subscriber[trafficontrol.ConnectionEvent]
	connectionEventObserver   *observable.Observer[trafficontrol.ConnectionEvent]
}

type ServiceOptions struct {
	Context context.Context
	// Platform           adapter.PlatformInterface
	Handler           PlatformHandler
	Debug             bool
	LogMaxLines       int
	OOMKillerEnabled  bool
	OOMKillerDisabled bool
	OOMMemoryLimit    uint64
	// WorkingDirectory   string
	// TempDirectory      string
	// UserID             int
	// GroupID            int
	// SystemProxyEnabled bool
}

func NewStartedService(options ServiceOptions) *StartedService {
	s := &StartedService{
		ctx: options.Context,
		// platform:                options.Platform,
		handler:           options.Handler,
		debug:             options.Debug,
		logMaxLines:       options.LogMaxLines,
		oomKillerEnabled:  options.OOMKillerEnabled,
		oomKillerDisabled: options.OOMKillerDisabled,
		oomMemoryLimit:    options.OOMMemoryLimit,
		// workingDirectory: options.WorkingDirectory,
		// tempDirectory:    options.TempDirectory,
		// userID:           options.UserID,
		// groupID:          options.GroupID,
		// systemProxyEnabled:      options.SystemProxyEnabled,
		serviceStatus:             &ServiceStatus{Status: ServiceStatus_IDLE},
		serviceStatusSubscriber:   observable.NewSubscriber[*ServiceStatus](4),
		logSubscriber:             observable.NewSubscriber[*log.Entry](128),
		urlTestSubscriber:         observable.NewSubscriber[struct{}](1),
		urlTestHistoryStorage:     urltest.NewHistoryStorage(),
		clashModeSubscriber:       observable.NewSubscriber[struct{}](1),
		connectionEventSubscriber: observable.NewSubscriber[trafficontrol.ConnectionEvent](256),
	}
	s.serviceStatusObserver = observable.NewObserver(s.serviceStatusSubscriber, 2)
	s.logObserver = observable.NewObserver(s.logSubscriber, 64)
	s.urlTestObserver = observable.NewObserver(s.urlTestSubscriber, 1)
	s.clashModeObserver = observable.NewObserver(s.clashModeSubscriber, 1)
	s.connectionEventObserver = observable.NewObserver(s.connectionEventSubscriber, 64)
	return s
}

func (s *StartedService) resetLogs() {
	s.logAccess.Lock()
	s.logLines = list.List[*log.Entry]{}
	s.logAccess.Unlock()
	s.logSubscriber.Emit(nil)
}

func (s *StartedService) updateStatus(newStatus ServiceStatus_Type) {
	statusObject := &ServiceStatus{Status: newStatus}
	s.serviceStatusSubscriber.Emit(statusObject)
	s.serviceStatus = statusObject
}

func (s *StartedService) updateStatusError(err error) error {
	statusObject := &ServiceStatus{Status: ServiceStatus_FATAL, ErrorMessage: err.Error()}
	s.serviceStatusSubscriber.Emit(statusObject)
	s.serviceStatus = statusObject
	s.serviceAccess.Unlock()
	return err
}

func (s *StartedService) waitForStarted(ctx context.Context) error {
	s.serviceAccess.RLock()
	currentStatus := s.serviceStatus.Status
	s.serviceAccess.RUnlock()

	switch currentStatus {
	case ServiceStatus_STARTED:
		return nil
	case ServiceStatus_STARTING:
	default:
		return os.ErrInvalid
	}

	subscription, done, err := s.serviceStatusObserver.Subscribe()
	if err != nil {
		return err
	}
	defer s.serviceStatusObserver.UnSubscribe(subscription)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.ctx.Done():
			return s.ctx.Err()
		case status := <-subscription:
			switch status.Status {
			case ServiceStatus_STARTED:
				return nil
			case ServiceStatus_FATAL:
				return E.New(status.ErrorMessage)
			case ServiceStatus_IDLE, ServiceStatus_STOPPING:
				return os.ErrInvalid
			}
		case <-done:
			return os.ErrClosed
		}
	}
}

func (s *StartedService) StartOrReloadService(profileContent string, options *OverrideOptions) error {
	s.serviceAccess.Lock()
	switch s.serviceStatus.Status {
	case ServiceStatus_IDLE, ServiceStatus_STARTED, ServiceStatus_STARTING, ServiceStatus_FATAL:
	default:
		s.serviceAccess.Unlock()
		return os.ErrInvalid
	}
	oldInstance := s.instance
	if oldInstance != nil {
		s.updateStatus(ServiceStatus_STOPPING)
		s.serviceAccess.Unlock()
		_ = oldInstance.Close()
		s.serviceAccess.Lock()
	}
	s.updateStatus(ServiceStatus_STARTING)
	s.resetLogs()
	instance, err := s.newInstance(profileContent, options)
	if err != nil {
		return s.updateStatusError(err)
	}
	s.instance = instance
	instance.urlTestHistoryStorage.SetHook(s.urlTestSubscriber)
	if instance.clashServer != nil {
		instance.clashServer.SetModeUpdateHook(s.clashModeSubscriber)
		instance.clashServer.(*clashapi.Server).TrafficManager().SetEventHook(s.connectionEventSubscriber)
	}
	s.serviceAccess.Unlock()
	err = instance.Start()
	s.serviceAccess.Lock()
	if s.serviceStatus.Status != ServiceStatus_STARTING {
		s.serviceAccess.Unlock()
		return nil
	}
	if err != nil {
		return s.updateStatusError(err)
	}
	s.startedAt = time.Now()
	s.updateStatus(ServiceStatus_STARTED)
	s.serviceAccess.Unlock()
	runtime.GC()
	return nil
}

func (s *StartedService) Close() {
	s.serviceStatusSubscriber.Close()
	s.logSubscriber.Close()
	s.urlTestSubscriber.Close()
	s.clashModeSubscriber.Close()
	s.connectionEventSubscriber.Close()
}

func (s *StartedService) CloseService() error {
	s.serviceAccess.Lock()
	switch s.serviceStatus.Status {
	case ServiceStatus_STARTING, ServiceStatus_STARTED:
	default:
		s.serviceAccess.Unlock()
		return os.ErrInvalid
	}
	s.updateStatus(ServiceStatus_STOPPING)
	instance := s.instance
	s.instance = nil
	if instance != nil {
		err := instance.Close()
		if err != nil {
			return s.updateStatusError(err)
		}
	}
	s.startedAt = time.Time{}
	s.updateStatus(ServiceStatus_IDLE)
	s.serviceAccess.Unlock()
	runtime.GC()
	return nil
}

func (s *StartedService) SetError(err error) {
	s.serviceAccess.Lock()
	s.updateStatusError(err)
	s.WriteMessage(log.LevelError, err.Error())
}

func (s *StartedService) StopService(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.handler.ServiceStop()
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) ReloadService(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	err := s.handler.ServiceReload()
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) SubscribeServiceStatus(empty *emptypb.Empty, server grpc.ServerStreamingServer[ServiceStatus]) error {
	subscription, done, err := s.serviceStatusObserver.Subscribe()
	if err != nil {
		return err
	}
	defer s.serviceStatusObserver.UnSubscribe(subscription)
	err = server.Send(s.serviceStatus)
	if err != nil {
		return err
	}
	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-server.Context().Done():
			return server.Context().Err()
		case newStatus := <-subscription:
			err = server.Send(newStatus)
			if err != nil {
				return err
			}
		case <-done:
			return nil
		}
	}
}

func (s *StartedService) SubscribeLog(empty *emptypb.Empty, server grpc.ServerStreamingServer[Log]) error {
	var savedLines []*log.Entry
	s.logAccess.Lock()
	savedLines = make([]*log.Entry, 0, s.logLines.Len())
	for element := s.logLines.Front(); element != nil; element = element.Next() {
		savedLines = append(savedLines, element.Value)
	}
	subscription, done, err := s.logObserver.Subscribe()
	s.logAccess.Unlock()
	if err != nil {
		return err
	}
	defer s.logObserver.UnSubscribe(subscription)
	err = server.Send(&Log{
		Messages: common.Map(savedLines, func(it *log.Entry) *Log_Message {
			return &Log_Message{
				Level:   LogLevel(it.Level),
				Message: it.Message,
			}
		}),
		Reset_: true,
	})
	if err != nil {
		return err
	}
	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-server.Context().Done():
			return server.Context().Err()
		case message := <-subscription:
			var rawMessage Log
			if message == nil {
				rawMessage.Reset_ = true
			} else {
				rawMessage.Messages = append(rawMessage.Messages, &Log_Message{
					Level:   LogLevel(message.Level),
					Message: message.Message,
				})
			}
		fetch:
			for {
				select {
				case message = <-subscription:
					if message == nil {
						rawMessage.Messages = nil
						rawMessage.Reset_ = true
					} else {
						rawMessage.Messages = append(rawMessage.Messages, &Log_Message{
							Level:   LogLevel(message.Level),
							Message: message.Message,
						})
					}
				default:
					break fetch
				}
			}
			err = server.Send(&rawMessage)
			if err != nil {
				return err
			}
		case <-done:
			return nil
		}
	}
}

func (s *StartedService) GetDefaultLogLevel(ctx context.Context, empty *emptypb.Empty) (*DefaultLogLevel, error) {
	s.serviceAccess.RLock()
	switch s.serviceStatus.Status {
	case ServiceStatus_STARTING, ServiceStatus_STARTED:
	default:
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	logLevel := s.instance.instance.LogFactory().Level()
	s.serviceAccess.RUnlock()
	return &DefaultLogLevel{Level: LogLevel(logLevel)}, nil
}

func (s *StartedService) ClearLogs(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.resetLogs()
	return &emptypb.Empty{}, nil
}

func (s *StartedService) SubscribeStatus(request *SubscribeStatusRequest, server grpc.ServerStreamingServer[Status]) error {
	interval := time.Duration(request.Interval)
	if interval <= 0 {
		interval = time.Second // Default to 1 second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	status := s.readStatus()
	uploadTotal := status.UplinkTotal
	downloadTotal := status.DownlinkTotal
	for {
		err := server.Send(status)
		if err != nil {
			return err
		}
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-server.Context().Done():
			return server.Context().Err()
		case <-ticker.C:
		}
		status = s.readStatus()
		upload := status.UplinkTotal - uploadTotal
		download := status.DownlinkTotal - downloadTotal
		uploadTotal = status.UplinkTotal
		downloadTotal = status.DownlinkTotal
		status.Uplink = upload
		status.Downlink = download
	}
}

func (s *StartedService) readStatus() *Status {
	var status Status
	status.Memory = memory.Total()
	status.Goroutines = int32(runtime.NumGoroutine())
	s.serviceAccess.RLock()
	nowService := s.instance
	s.serviceAccess.RUnlock()
	if nowService != nil && nowService.connectionManager != nil {
		status.ConnectionsOut = int32(nowService.connectionManager.Count())
	}
	if nowService != nil {
		if clashServer := nowService.clashServer; clashServer != nil {
			status.TrafficAvailable = true
			trafficManager := clashServer.(*clashapi.Server).TrafficManager()
			status.UplinkTotal, status.DownlinkTotal = trafficManager.Total()
			status.ConnectionsIn = int32(trafficManager.ConnectionsLen())
		}
	}
	return &status
}

func (s *StartedService) SubscribeGroups(empty *emptypb.Empty, server grpc.ServerStreamingServer[Groups]) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	subscription, done, err := s.urlTestObserver.Subscribe()
	if err != nil {
		return err
	}
	defer s.urlTestObserver.UnSubscribe(subscription)
	for {
		s.serviceAccess.RLock()
		if s.serviceStatus.Status != ServiceStatus_STARTED {
			s.serviceAccess.RUnlock()
			return os.ErrInvalid
		}
		groups := s.readGroups()
		s.serviceAccess.RUnlock()
		err = server.Send(groups)
		if err != nil {
			return err
		}
		select {
		case <-subscription:
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-server.Context().Done():
			return server.Context().Err()
		case <-done:
			return nil
		}
	}
}

func (s *StartedService) readGroups() *Groups {
	historyStorage := s.instance.urlTestHistoryStorage
	boxService := s.instance
	outbounds := boxService.instance.Outbound().Outbounds()
	var iGroups []adapter.OutboundGroup
	for _, it := range outbounds {
		if group, isGroup := it.(adapter.OutboundGroup); isGroup {
			iGroups = append(iGroups, group)
		}
	}
	var gs Groups
	for _, iGroup := range iGroups {
		var g Group
		g.Tag = iGroup.Tag()
		g.Type = iGroup.Type()
		_, g.Selectable = iGroup.(*group.Selector)
		g.Selected = iGroup.Now()
		if boxService.cacheFile != nil {
			if isExpand, loaded := boxService.cacheFile.LoadGroupExpand(g.Tag); loaded {
				g.IsExpand = isExpand
			}
		}

		for _, itemTag := range iGroup.All() {
			itemOutbound, isLoaded := boxService.instance.Outbound().Outbound(itemTag)
			if !isLoaded {
				continue
			}

			var item GroupItem
			item.Tag = itemTag
			item.Type = itemOutbound.Type()
			if history := historyStorage.LoadURLTestHistory(adapter.OutboundTag(itemOutbound)); history != nil {
				item.UrlTestTime = history.Time.Unix()
				item.UrlTestDelay = int32(history.Delay)
			}
			g.Items = append(g.Items, &item)
		}
		if len(g.Items) < 2 {
			continue
		}
		gs.Group = append(gs.Group, &g)
	}
	return &gs
}

func (s *StartedService) GetClashModeStatus(ctx context.Context, empty *emptypb.Empty) (*ClashModeStatus, error) {
	s.serviceAccess.RLock()
	if s.serviceStatus.Status != ServiceStatus_STARTED {
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	clashServer := s.instance.clashServer
	s.serviceAccess.RUnlock()
	if clashServer == nil {
		return nil, os.ErrInvalid
	}
	return &ClashModeStatus{
		ModeList:    clashServer.ModeList(),
		CurrentMode: clashServer.Mode(),
	}, nil
}

func (s *StartedService) SubscribeClashMode(empty *emptypb.Empty, server grpc.ServerStreamingServer[ClashMode]) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	subscription, done, err := s.clashModeObserver.Subscribe()
	if err != nil {
		return err
	}
	defer s.clashModeObserver.UnSubscribe(subscription)
	for {
		s.serviceAccess.RLock()
		if s.serviceStatus.Status != ServiceStatus_STARTED {
			s.serviceAccess.RUnlock()
			return os.ErrInvalid
		}
		message := &ClashMode{Mode: s.instance.clashServer.Mode()}
		s.serviceAccess.RUnlock()
		err = server.Send(message)
		if err != nil {
			return err
		}
		select {
		case <-subscription:
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-server.Context().Done():
			return server.Context().Err()
		case <-done:
			return nil
		}
	}
}

func (s *StartedService) SetClashMode(ctx context.Context, request *ClashMode) (*emptypb.Empty, error) {
	s.serviceAccess.RLock()
	if s.serviceStatus.Status != ServiceStatus_STARTED {
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	clashServer := s.instance.clashServer
	s.serviceAccess.RUnlock()
	clashServer.(*clashapi.Server).SetMode(request.Mode)
	return &emptypb.Empty{}, nil
}

func (s *StartedService) URLTest(ctx context.Context, request *URLTestRequest) (*emptypb.Empty, error) {
	s.serviceAccess.RLock()
	if s.serviceStatus.Status != ServiceStatus_STARTED {
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	boxService := s.instance
	s.serviceAccess.RUnlock()
	groupTag := request.OutboundTag
	abstractOutboundGroup, isLoaded := boxService.instance.Outbound().Outbound(groupTag)
	if !isLoaded {
		return nil, E.New("outbound group not found: ", groupTag)
	}
	outboundGroup, isOutboundGroup := abstractOutboundGroup.(adapter.OutboundGroup)
	if !isOutboundGroup {
		return nil, E.New("outbound is not a group: ", groupTag)
	}
	urlTest, isURLTest := abstractOutboundGroup.(*group.URLTest)
	if isURLTest {
		go urlTest.CheckOutbounds()
	} else {
		historyStorage := boxService.urlTestHistoryStorage

		outbounds := common.Filter(common.Map(outboundGroup.All(), func(it string) adapter.Outbound {
			itOutbound, _ := boxService.instance.Outbound().Outbound(it)
			return itOutbound
		}), func(it adapter.Outbound) bool {
			if it == nil {
				return false
			}
			_, isGroup := it.(adapter.OutboundGroup)
			if isGroup {
				return false
			}
			return true
		})
		b, _ := batch.New(boxService.ctx, batch.WithConcurrencyNum[any](10))
		for _, detour := range outbounds {
			outboundToTest := detour
			outboundTag := outboundToTest.Tag()
			b.Go(outboundTag, func() (any, error) {
				t, err := urltest.URLTest(boxService.ctx, "", outboundToTest)
				if err != nil {
					historyStorage.DeleteURLTestHistory(outboundTag)
				} else {
					historyStorage.StoreURLTestHistory(outboundTag, &adapter.URLTestHistory{
						Time:  time.Now(),
						Delay: t,
					})
				}
				return nil, nil
			})
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) SelectOutbound(ctx context.Context, request *SelectOutboundRequest) (*emptypb.Empty, error) {
	s.serviceAccess.RLock()
	switch s.serviceStatus.Status {
	case ServiceStatus_STARTING, ServiceStatus_STARTED:
	default:
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	boxService := s.instance.instance
	s.serviceAccess.RUnlock()
	outboundGroup, isLoaded := boxService.Outbound().Outbound(request.GroupTag)
	if !isLoaded {
		return nil, E.New("selector not found: ", request.GroupTag)
	}
	selector, isSelector := outboundGroup.(*group.Selector)
	if !isSelector {
		return nil, E.New("outbound is not a selector: ", request.GroupTag)
	}
	if !selector.SelectOutbound(request.OutboundTag) {
		return nil, E.New("outbound not found in selector: ", request.OutboundTag)
	}
	s.urlTestObserver.Emit(struct{}{})
	return &emptypb.Empty{}, nil
}

func (s *StartedService) SetGroupExpand(ctx context.Context, request *SetGroupExpandRequest) (*emptypb.Empty, error) {
	s.serviceAccess.RLock()
	switch s.serviceStatus.Status {
	case ServiceStatus_STARTING, ServiceStatus_STARTED:
	default:
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	boxService := s.instance
	s.serviceAccess.RUnlock()
	if boxService.cacheFile != nil {
		err := boxService.cacheFile.StoreGroupExpand(request.GroupTag, request.IsExpand)
		if err != nil {
			return nil, err
		}
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) GetSystemProxyStatus(ctx context.Context, empty *emptypb.Empty) (*SystemProxyStatus, error) {
	return s.handler.SystemProxyStatus()
}

func (s *StartedService) SetSystemProxyEnabled(ctx context.Context, request *SetSystemProxyEnabledRequest) (*emptypb.Empty, error) {
	err := s.handler.SetSystemProxyEnabled(request.Enabled)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) TriggerDebugCrash(ctx context.Context, request *DebugCrashRequest) (*emptypb.Empty, error) {
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

func (s *StartedService) TriggerOOMReport(ctx context.Context, _ *emptypb.Empty) (*emptypb.Empty, error) {
	instance := s.Instance()
	if instance == nil {
		return nil, status.Error(codes.FailedPrecondition, "service not started")
	}
	reporter := service.FromContext[oomkiller.OOMReporter](instance.ctx)
	if reporter == nil {
		return nil, status.Error(codes.Unavailable, "OOM reporter not available")
	}
	return &emptypb.Empty{}, reporter.WriteReport(memory.Total())
}

func (s *StartedService) SubscribeConnections(request *SubscribeConnectionsRequest, server grpc.ServerStreamingServer[ConnectionEvents]) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	if boxService.clashServer == nil {
		return E.New("clash server not available")
	}

	trafficManager := boxService.clashServer.(*clashapi.Server).TrafficManager()

	subscription, done, err := s.connectionEventObserver.Subscribe()
	if err != nil {
		return err
	}
	defer s.connectionEventObserver.UnSubscribe(subscription)

	connectionSnapshots := make(map[uuid.UUID]connectionSnapshot)
	initialEvents := s.buildInitialConnectionState(trafficManager, connectionSnapshots)
	err = server.Send(&ConnectionEvents{
		Events: initialEvents,
		Reset_: true,
	})
	if err != nil {
		return err
	}

	interval := time.Duration(request.Interval)
	if interval <= 0 {
		interval = time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-server.Context().Done():
			return server.Context().Err()
		case <-done:
			return nil

		case event := <-subscription:
			var pendingEvents []*ConnectionEvent
			if protoEvent := s.applyConnectionEvent(event, connectionSnapshots); protoEvent != nil {
				pendingEvents = append(pendingEvents, protoEvent)
			}
		drain:
			for {
				select {
				case event = <-subscription:
					if protoEvent := s.applyConnectionEvent(event, connectionSnapshots); protoEvent != nil {
						pendingEvents = append(pendingEvents, protoEvent)
					}
				default:
					break drain
				}
			}
			if len(pendingEvents) > 0 {
				err = server.Send(&ConnectionEvents{Events: pendingEvents})
				if err != nil {
					return err
				}
			}

		case <-ticker.C:
			protoEvents := s.buildTrafficUpdates(trafficManager, connectionSnapshots)
			if len(protoEvents) == 0 {
				continue
			}
			err = server.Send(&ConnectionEvents{Events: protoEvents})
			if err != nil {
				return err
			}
		}
	}
}

type connectionSnapshot struct {
	uplink     int64
	downlink   int64
	hadTraffic bool
}

func (s *StartedService) buildInitialConnectionState(manager *trafficontrol.Manager, snapshots map[uuid.UUID]connectionSnapshot) []*ConnectionEvent {
	var events []*ConnectionEvent

	for _, metadata := range manager.Connections() {
		events = append(events, &ConnectionEvent{
			Type:       ConnectionEventType_CONNECTION_EVENT_NEW,
			Id:         metadata.ID.String(),
			Connection: buildConnectionProto(metadata),
		})
		snapshots[metadata.ID] = connectionSnapshot{
			uplink:   metadata.Upload.Load(),
			downlink: metadata.Download.Load(),
		}
	}

	for _, metadata := range manager.ClosedConnections() {
		conn := buildConnectionProto(metadata)
		conn.ClosedAt = metadata.ClosedAt.UnixMilli()
		events = append(events, &ConnectionEvent{
			Type:       ConnectionEventType_CONNECTION_EVENT_NEW,
			Id:         metadata.ID.String(),
			Connection: conn,
		})
	}

	return events
}

func (s *StartedService) applyConnectionEvent(event trafficontrol.ConnectionEvent, snapshots map[uuid.UUID]connectionSnapshot) *ConnectionEvent {
	switch event.Type {
	case trafficontrol.ConnectionEventNew:
		if _, exists := snapshots[event.ID]; exists {
			return nil
		}
		snapshots[event.ID] = connectionSnapshot{
			uplink:   event.Metadata.Upload.Load(),
			downlink: event.Metadata.Download.Load(),
		}
		return &ConnectionEvent{
			Type:       ConnectionEventType_CONNECTION_EVENT_NEW,
			Id:         event.ID.String(),
			Connection: buildConnectionProto(event.Metadata),
		}
	case trafficontrol.ConnectionEventClosed:
		delete(snapshots, event.ID)
		protoEvent := &ConnectionEvent{
			Type: ConnectionEventType_CONNECTION_EVENT_CLOSED,
			Id:   event.ID.String(),
		}
		closedAt := event.ClosedAt
		if closedAt.IsZero() && !event.Metadata.ClosedAt.IsZero() {
			closedAt = event.Metadata.ClosedAt
		}
		if closedAt.IsZero() {
			closedAt = time.Now()
		}
		protoEvent.ClosedAt = closedAt.UnixMilli()
		if event.Metadata.ID != uuid.Nil {
			conn := buildConnectionProto(event.Metadata)
			conn.ClosedAt = protoEvent.ClosedAt
			protoEvent.Connection = conn
		}
		return protoEvent
	default:
		return nil
	}
}

func (s *StartedService) buildTrafficUpdates(manager *trafficontrol.Manager, snapshots map[uuid.UUID]connectionSnapshot) []*ConnectionEvent {
	activeConnections := manager.Connections()
	activeIndex := make(map[uuid.UUID]*trafficontrol.TrackerMetadata, len(activeConnections))
	var events []*ConnectionEvent

	for _, metadata := range activeConnections {
		activeIndex[metadata.ID] = metadata
		currentUpload := metadata.Upload.Load()
		currentDownload := metadata.Download.Load()
		snapshot, exists := snapshots[metadata.ID]
		if !exists {
			snapshots[metadata.ID] = connectionSnapshot{
				uplink:   currentUpload,
				downlink: currentDownload,
			}
			events = append(events, &ConnectionEvent{
				Type:       ConnectionEventType_CONNECTION_EVENT_NEW,
				Id:         metadata.ID.String(),
				Connection: buildConnectionProto(metadata),
			})
			continue
		}
		uplinkDelta := currentUpload - snapshot.uplink
		downlinkDelta := currentDownload - snapshot.downlink
		if uplinkDelta < 0 || downlinkDelta < 0 {
			if snapshot.hadTraffic {
				events = append(events, &ConnectionEvent{
					Type:          ConnectionEventType_CONNECTION_EVENT_UPDATE,
					Id:            metadata.ID.String(),
					UplinkDelta:   0,
					DownlinkDelta: 0,
				})
			}
			snapshot.uplink = currentUpload
			snapshot.downlink = currentDownload
			snapshot.hadTraffic = false
			snapshots[metadata.ID] = snapshot
			continue
		}
		if uplinkDelta > 0 || downlinkDelta > 0 {
			snapshot.uplink = currentUpload
			snapshot.downlink = currentDownload
			snapshot.hadTraffic = true
			snapshots[metadata.ID] = snapshot
			events = append(events, &ConnectionEvent{
				Type:          ConnectionEventType_CONNECTION_EVENT_UPDATE,
				Id:            metadata.ID.String(),
				UplinkDelta:   uplinkDelta,
				DownlinkDelta: downlinkDelta,
			})
			continue
		}
		if snapshot.hadTraffic {
			snapshot.uplink = currentUpload
			snapshot.downlink = currentDownload
			snapshot.hadTraffic = false
			snapshots[metadata.ID] = snapshot
			events = append(events, &ConnectionEvent{
				Type:          ConnectionEventType_CONNECTION_EVENT_UPDATE,
				Id:            metadata.ID.String(),
				UplinkDelta:   0,
				DownlinkDelta: 0,
			})
		}
	}

	var closedIndex map[uuid.UUID]*trafficontrol.TrackerMetadata
	for id := range snapshots {
		if _, exists := activeIndex[id]; exists {
			continue
		}
		if closedIndex == nil {
			closedIndex = make(map[uuid.UUID]*trafficontrol.TrackerMetadata)
			for _, metadata := range manager.ClosedConnections() {
				closedIndex[metadata.ID] = metadata
			}
		}
		closedAt := time.Now()
		var conn *Connection
		if metadata, ok := closedIndex[id]; ok {
			if !metadata.ClosedAt.IsZero() {
				closedAt = metadata.ClosedAt
			}
			conn = buildConnectionProto(metadata)
			conn.ClosedAt = closedAt.UnixMilli()
		}
		events = append(events, &ConnectionEvent{
			Type:       ConnectionEventType_CONNECTION_EVENT_CLOSED,
			Id:         id.String(),
			ClosedAt:   closedAt.UnixMilli(),
			Connection: conn,
		})
		delete(snapshots, id)
	}

	return events
}

func buildConnectionProto(metadata *trafficontrol.TrackerMetadata) *Connection {
	var rule string
	if metadata.Rule != nil {
		rule = metadata.Rule.String()
	}
	uplinkTotal := metadata.Upload.Load()
	downlinkTotal := metadata.Download.Load()
	var processInfo *ProcessInfo
	if metadata.Metadata.ProcessInfo != nil {
		processInfo = &ProcessInfo{
			ProcessId:    metadata.Metadata.ProcessInfo.ProcessID,
			UserId:       metadata.Metadata.ProcessInfo.UserId,
			UserName:     metadata.Metadata.ProcessInfo.UserName,
			ProcessPath:  metadata.Metadata.ProcessInfo.ProcessPath,
			PackageNames: metadata.Metadata.ProcessInfo.AndroidPackageNames,
		}
	}
	return &Connection{
		Id:            metadata.ID.String(),
		Inbound:       metadata.Metadata.Inbound,
		InboundType:   metadata.Metadata.InboundType,
		IpVersion:     int32(metadata.Metadata.IPVersion),
		Network:       metadata.Metadata.Network,
		Source:        metadata.Metadata.Source.String(),
		Destination:   metadata.Metadata.Destination.String(),
		Domain:        metadata.Metadata.Domain,
		Protocol:      metadata.Metadata.Protocol,
		User:          metadata.Metadata.User,
		FromOutbound:  metadata.Metadata.Outbound,
		CreatedAt:     metadata.CreatedAt.UnixMilli(),
		UplinkTotal:   uplinkTotal,
		DownlinkTotal: downlinkTotal,
		Rule:          rule,
		Outbound:      metadata.Outbound,
		OutboundType:  metadata.OutboundType,
		ChainList:     metadata.Chain,
		ProcessInfo:   processInfo,
	}
}

func (s *StartedService) CloseConnection(ctx context.Context, request *CloseConnectionRequest) (*emptypb.Empty, error) {
	s.serviceAccess.RLock()
	switch s.serviceStatus.Status {
	case ServiceStatus_STARTING, ServiceStatus_STARTED:
	default:
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	boxService := s.instance
	s.serviceAccess.RUnlock()
	targetConn := boxService.clashServer.(*clashapi.Server).TrafficManager().Connection(uuid.FromStringOrNil(request.Id))
	if targetConn != nil {
		targetConn.Close()
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) CloseAllConnections(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	s.serviceAccess.RLock()
	nowService := s.instance
	s.serviceAccess.RUnlock()
	if nowService != nil && nowService.connectionManager != nil {
		nowService.connectionManager.CloseAll()
	}
	return &emptypb.Empty{}, nil
}

func (s *StartedService) GetDeprecatedWarnings(ctx context.Context, empty *emptypb.Empty) (*DeprecatedWarnings, error) {
	s.serviceAccess.RLock()
	if s.serviceStatus.Status != ServiceStatus_STARTED {
		s.serviceAccess.RUnlock()
		return nil, os.ErrInvalid
	}
	boxService := s.instance
	s.serviceAccess.RUnlock()
	notes := service.FromContext[deprecated.Manager](boxService.ctx).(*deprecatedManager).Get()
	return &DeprecatedWarnings{
		Warnings: common.Map(notes, func(it deprecated.Note) *DeprecatedWarning {
			return &DeprecatedWarning{
				Message:           it.Message(),
				Impending:         it.Impending(),
				MigrationLink:     it.MigrationLink,
				Description:       it.Description,
				DeprecatedVersion: it.DeprecatedVersion,
				ScheduledVersion:  it.ScheduledVersion,
			}
		}),
	}, nil
}

func (s *StartedService) GetStartedAt(ctx context.Context, empty *emptypb.Empty) (*StartedAt, error) {
	s.serviceAccess.RLock()
	defer s.serviceAccess.RUnlock()
	return &StartedAt{StartedAt: s.startedAt.UnixMilli()}, nil
}

func (s *StartedService) SubscribeOutbounds(_ *emptypb.Empty, server grpc.ServerStreamingServer[OutboundList]) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	subscription, done, err := s.urlTestObserver.Subscribe()
	if err != nil {
		return err
	}
	defer s.urlTestObserver.UnSubscribe(subscription)
	for {
		s.serviceAccess.RLock()
		if s.serviceStatus.Status != ServiceStatus_STARTED {
			s.serviceAccess.RUnlock()
			return os.ErrInvalid
		}
		boxService := s.instance
		s.serviceAccess.RUnlock()
		historyStorage := boxService.urlTestHistoryStorage
		var list OutboundList
		for _, ob := range boxService.instance.Outbound().Outbounds() {
			item := &GroupItem{
				Tag:  ob.Tag(),
				Type: ob.Type(),
			}
			if history := historyStorage.LoadURLTestHistory(adapter.OutboundTag(ob)); history != nil {
				item.UrlTestTime = history.Time.Unix()
				item.UrlTestDelay = int32(history.Delay)
			}
			list.Outbounds = append(list.Outbounds, item)
		}
		for _, ep := range boxService.instance.Endpoint().Endpoints() {
			item := &GroupItem{
				Tag:  ep.Tag(),
				Type: ep.Type(),
			}
			if history := historyStorage.LoadURLTestHistory(adapter.OutboundTag(ep)); history != nil {
				item.UrlTestTime = history.Time.Unix()
				item.UrlTestDelay = int32(history.Delay)
			}
			list.Outbounds = append(list.Outbounds, item)
		}
		err = server.Send(&list)
		if err != nil {
			return err
		}
		select {
		case <-subscription:
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-server.Context().Done():
			return server.Context().Err()
		case <-done:
			return nil
		}
	}
}

func resolveOutbound(instance *Instance, tag string) (adapter.Outbound, error) {
	if tag == "" {
		return instance.instance.Outbound().Default(), nil
	}
	outbound, loaded := instance.instance.Outbound().Outbound(tag)
	if !loaded {
		return nil, E.New("outbound not found: ", tag)
	}
	return outbound, nil
}

func (s *StartedService) StartNetworkQualityTest(
	request *NetworkQualityTestRequest,
	server grpc.ServerStreamingServer[NetworkQualityTestProgress],
) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	outbound, err := resolveOutbound(boxService, request.OutboundTag)
	if err != nil {
		return err
	}

	resolvedDialer := dialer.NewResolveDialer(boxService.ctx, outbound, true, "", adapter.DNSQueryOptions{}, 0)
	httpClient := networkquality.NewHTTPClient(resolvedDialer)
	defer httpClient.CloseIdleConnections()

	measurementClientFactory, err := networkquality.NewOptionalHTTP3Factory(resolvedDialer, request.Http3)
	if err != nil {
		return err
	}

	result, nqErr := networkquality.Run(networkquality.Options{
		ConfigURL:            request.ConfigURL,
		HTTPClient:           httpClient,
		NewMeasurementClient: measurementClientFactory,
		Serial:               request.Serial,
		MaxRuntime:           time.Duration(request.MaxRuntimeSeconds) * time.Second,
		Context:              server.Context(),
		OnProgress: func(p networkquality.Progress) {
			_ = server.Send(&NetworkQualityTestProgress{
				Phase:                    int32(p.Phase),
				DownloadCapacity:         p.DownloadCapacity,
				UploadCapacity:           p.UploadCapacity,
				DownloadRPM:              p.DownloadRPM,
				UploadRPM:                p.UploadRPM,
				IdleLatencyMs:            p.IdleLatencyMs,
				ElapsedMs:                p.ElapsedMs,
				DownloadCapacityAccuracy: int32(p.DownloadCapacityAccuracy),
				UploadCapacityAccuracy:   int32(p.UploadCapacityAccuracy),
				DownloadRPMAccuracy:      int32(p.DownloadRPMAccuracy),
				UploadRPMAccuracy:        int32(p.UploadRPMAccuracy),
			})
		},
	})
	if nqErr != nil {
		return server.Send(&NetworkQualityTestProgress{
			IsFinal: true,
			Error:   nqErr.Error(),
		})
	}
	return server.Send(&NetworkQualityTestProgress{
		Phase:                    int32(networkquality.PhaseDone),
		DownloadCapacity:         result.DownloadCapacity,
		UploadCapacity:           result.UploadCapacity,
		DownloadRPM:              result.DownloadRPM,
		UploadRPM:                result.UploadRPM,
		IdleLatencyMs:            result.IdleLatencyMs,
		IsFinal:                  true,
		DownloadCapacityAccuracy: int32(result.DownloadCapacityAccuracy),
		UploadCapacityAccuracy:   int32(result.UploadCapacityAccuracy),
		DownloadRPMAccuracy:      int32(result.DownloadRPMAccuracy),
		UploadRPMAccuracy:        int32(result.UploadRPMAccuracy),
	})
}

func (s *StartedService) StartSTUNTest(
	request *STUNTestRequest,
	server grpc.ServerStreamingServer[STUNTestProgress],
) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	outbound, err := resolveOutbound(boxService, request.OutboundTag)
	if err != nil {
		return err
	}

	resolvedDialer := dialer.NewResolveDialer(boxService.ctx, outbound, true, "", adapter.DNSQueryOptions{}, 0)

	result, stunErr := stun.Run(stun.Options{
		Server:  request.Server,
		Dialer:  resolvedDialer,
		Context: server.Context(),
		OnProgress: func(p stun.Progress) {
			_ = server.Send(&STUNTestProgress{
				Phase:        int32(p.Phase),
				ExternalAddr: p.ExternalAddr,
				LatencyMs:    p.LatencyMs,
				NatMapping:   int32(p.NATMapping),
				NatFiltering: int32(p.NATFiltering),
			})
		},
	})
	if stunErr != nil {
		return server.Send(&STUNTestProgress{
			IsFinal: true,
			Error:   stunErr.Error(),
		})
	}
	return server.Send(&STUNTestProgress{
		Phase:            int32(stun.PhaseDone),
		ExternalAddr:     result.ExternalAddr,
		LatencyMs:        result.LatencyMs,
		NatMapping:       int32(result.NATMapping),
		NatFiltering:     int32(result.NATFiltering),
		IsFinal:          true,
		NatTypeSupported: result.NATTypeSupported,
	})
}

func (s *StartedService) SubscribeTailscaleStatus(
	_ *emptypb.Empty,
	server grpc.ServerStreamingServer[TailscaleStatusUpdate],
) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	endpointManager := service.FromContext[adapter.EndpointManager](boxService.ctx)
	if endpointManager == nil {
		return status.Error(codes.FailedPrecondition, "endpoint manager not available")
	}

	type tailscaleEndpoint struct {
		tag      string
		provider adapter.TailscaleEndpoint
	}
	var endpoints []tailscaleEndpoint
	for _, endpoint := range endpointManager.Endpoints() {
		if endpoint.Type() != C.TypeTailscale {
			continue
		}
		provider, loaded := endpoint.(adapter.TailscaleEndpoint)
		if !loaded {
			continue
		}
		endpoints = append(endpoints, tailscaleEndpoint{
			tag:      endpoint.Tag(),
			provider: provider,
		})
	}
	if len(endpoints) == 0 {
		return status.Error(codes.NotFound, "no Tailscale endpoint found")
	}

	type taggedStatus struct {
		tag    string
		status *adapter.TailscaleEndpointStatus
	}
	updates := make(chan taggedStatus, len(endpoints))
	ctx, cancel := context.WithCancel(server.Context())
	defer cancel()

	var waitGroup sync.WaitGroup
	for _, endpoint := range endpoints {
		waitGroup.Add(1)
		go func(tag string, provider adapter.TailscaleEndpoint) {
			defer waitGroup.Done()
			_ = provider.SubscribeTailscaleStatus(ctx, func(endpointStatus *adapter.TailscaleEndpointStatus) {
				select {
				case updates <- taggedStatus{tag: tag, status: endpointStatus}:
				case <-ctx.Done():
				}
			})
		}(endpoint.tag, endpoint.provider)
	}

	go func() {
		waitGroup.Wait()
		close(updates)
	}()

	var tags []string
	statuses := make(map[string]*adapter.TailscaleEndpointStatus, len(endpoints))
	for update := range updates {
		if _, exists := statuses[update.tag]; !exists {
			tags = append(tags, update.tag)
		}
		statuses[update.tag] = update.status
		protoEndpoints := make([]*TailscaleEndpointStatus, 0, len(statuses))
		for _, tag := range tags {
			protoEndpoints = append(protoEndpoints, tailscaleEndpointStatusToProto(tag, statuses[tag]))
		}
		sendErr := server.Send(&TailscaleStatusUpdate{
			Endpoints: protoEndpoints,
		})
		if sendErr != nil {
			return sendErr
		}
	}
	return nil
}

func tailscaleEndpointStatusToProto(tag string, s *adapter.TailscaleEndpointStatus) *TailscaleEndpointStatus {
	userGroups := make([]*TailscaleUserGroup, len(s.UserGroups))
	for i, group := range s.UserGroups {
		peers := make([]*TailscalePeer, len(group.Peers))
		for j, peer := range group.Peers {
			peers[j] = tailscalePeerToProto(peer)
		}
		userGroups[i] = &TailscaleUserGroup{
			UserID:        group.UserID,
			LoginName:     group.LoginName,
			DisplayName:   group.DisplayName,
			ProfilePicURL: group.ProfilePicURL,
			Peers:         peers,
		}
	}
	result := &TailscaleEndpointStatus{
		EndpointTag:    tag,
		BackendState:   s.BackendState,
		AuthURL:        s.AuthURL,
		NetworkName:    s.NetworkName,
		MagicDNSSuffix: s.MagicDNSSuffix,
		UserGroups:     userGroups,
	}
	if s.Self != nil {
		result.Self = tailscalePeerToProto(s.Self)
	}
	return result
}

func tailscalePeerToProto(peer *adapter.TailscalePeer) *TailscalePeer {
	return &TailscalePeer{
		HostName:       peer.HostName,
		DnsName:        peer.DNSName,
		Os:             peer.OS,
		TailscaleIPs:   peer.TailscaleIPs,
		Online:         peer.Online,
		ExitNode:       peer.ExitNode,
		ExitNodeOption: peer.ExitNodeOption,
		Active:         peer.Active,
		RxBytes:        peer.RxBytes,
		TxBytes:        peer.TxBytes,
		KeyExpiry:      peer.KeyExpiry,
	}
}

func (s *StartedService) StartTailscalePing(
	request *TailscalePingRequest,
	server grpc.ServerStreamingServer[TailscalePingResponse],
) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()

	endpointManager := service.FromContext[adapter.EndpointManager](boxService.ctx)
	if endpointManager == nil {
		return status.Error(codes.FailedPrecondition, "endpoint manager not available")
	}

	var provider adapter.TailscaleEndpoint
	if request.EndpointTag != "" {
		endpoint, loaded := endpointManager.Get(request.EndpointTag)
		if !loaded {
			return status.Error(codes.NotFound, "endpoint not found: "+request.EndpointTag)
		}
		if endpoint.Type() != C.TypeTailscale {
			return status.Error(codes.InvalidArgument, "endpoint is not Tailscale: "+request.EndpointTag)
		}
		pingProvider, loaded := endpoint.(adapter.TailscaleEndpoint)
		if !loaded {
			return status.Error(codes.FailedPrecondition, "endpoint does not support ping")
		}
		provider = pingProvider
	} else {
		for _, endpoint := range endpointManager.Endpoints() {
			if endpoint.Type() != C.TypeTailscale {
				continue
			}
			pingProvider, loaded := endpoint.(adapter.TailscaleEndpoint)
			if loaded {
				provider = pingProvider
				break
			}
		}
		if provider == nil {
			return status.Error(codes.NotFound, "no Tailscale endpoint found")
		}
	}

	return provider.StartTailscalePing(server.Context(), request.PeerIP, func(result *adapter.TailscalePingResult) {
		_ = server.Send(&TailscalePingResponse{
			LatencyMs:      result.LatencyMs,
			IsDirect:       result.IsDirect,
			Endpoint:       result.Endpoint,
			DerpRegionID:   result.DERPRegionID,
			DerpRegionCode: result.DERPRegionCode,
			Error:          result.Error,
		})
	})
}

func (s *StartedService) mustEmbedUnimplementedStartedServiceServer() {
}

func (s *StartedService) WriteMessage(level log.Level, message string) {
	item := &log.Entry{Level: level, Message: message}
	s.logAccess.Lock()
	s.logLines.PushBack(item)
	if s.logLines.Len() > s.logMaxLines {
		s.logLines.Remove(s.logLines.Front())
	}
	s.logAccess.Unlock()
	s.logSubscriber.Emit(item)
	if s.debug {
		s.handler.WriteDebugMessage(message)
	}
}

func (s *StartedService) Instance() *Instance {
	s.serviceAccess.RLock()
	defer s.serviceAccess.RUnlock()
	return s.instance
}
