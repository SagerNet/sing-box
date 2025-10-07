package daemon

import (
	"context"
	"os"
	"runtime"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/conntrack"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/experimental/clashapi"
	"github.com/sagernet/sing-box/experimental/clashapi/trafficontrol"
	"github.com/sagernet/sing-box/experimental/deprecated"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/protocol/group"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/memory"
	"github.com/sagernet/sing/common/observable"
	"github.com/sagernet/sing/common/x/list"
	"github.com/sagernet/sing/service"

	"github.com/gofrs/uuid/v5"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

var _ StartedServiceServer = (*StartedService)(nil)

type StartedService struct {
	ctx context.Context
	// platform adapter.PlatformInterface
	handler     PlatformHandler
	debug       bool
	logMaxLines int
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
	urlTestSubscriber       *observable.Subscriber[struct{}]
	urlTestObserver         *observable.Observer[struct{}]
	urlTestHistoryStorage   *urltest.HistoryStorage
	clashModeSubscriber     *observable.Subscriber[struct{}]
	clashModeObserver       *observable.Observer[struct{}]
}

type ServiceOptions struct {
	Context context.Context
	// Platform           adapter.PlatformInterface
	Handler     PlatformHandler
	Debug       bool
	LogMaxLines int
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
		handler:     options.Handler,
		debug:       options.Debug,
		logMaxLines: options.LogMaxLines,
		// workingDirectory: options.WorkingDirectory,
		// tempDirectory:    options.TempDirectory,
		// userID:           options.UserID,
		// groupID:          options.GroupID,
		// systemProxyEnabled:      options.SystemProxyEnabled,
		serviceStatus:           &ServiceStatus{Status: ServiceStatus_IDLE},
		serviceStatusSubscriber: observable.NewSubscriber[*ServiceStatus](4),
		logSubscriber:           observable.NewSubscriber[*log.Entry](128),
		urlTestSubscriber:       observable.NewSubscriber[struct{}](1),
		urlTestHistoryStorage:   urltest.NewHistoryStorage(),
		clashModeSubscriber:     observable.NewSubscriber[struct{}](1),
	}
	s.serviceStatusObserver = observable.NewObserver(s.serviceStatusSubscriber, 2)
	s.logObserver = observable.NewObserver(s.logSubscriber, 64)
	s.urlTestObserver = observable.NewObserver(s.urlTestSubscriber, 1)
	s.clashModeObserver = observable.NewObserver(s.clashModeSubscriber, 1)
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
	case ServiceStatus_IDLE, ServiceStatus_STARTED, ServiceStatus_STARTING:
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
	s.updateStatus(ServiceStatus_STARTED)
	s.serviceAccess.Unlock()
	runtime.GC()
	return nil
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
	if s.instance != nil {
		err := s.instance.Close()
		if err != nil {
			return s.updateStatusError(err)
		}
	}
	s.instance = nil
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
	s.logAccess.Unlock()
	subscription, done, err := s.logObserver.Subscribe()
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
	status.Memory = memory.Inuse()
	status.Goroutines = int32(runtime.NumGoroutine())
	status.ConnectionsOut = int32(conntrack.Count())
	s.serviceAccess.RLock()
	nowService := s.instance
	s.serviceAccess.RUnlock()
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
	return nil, err
}

func (s *StartedService) SubscribeConnections(request *SubscribeConnectionsRequest, server grpc.ServerStreamingServer[Connections]) error {
	err := s.waitForStarted(server.Context())
	if err != nil {
		return err
	}
	s.serviceAccess.RLock()
	boxService := s.instance
	s.serviceAccess.RUnlock()
	ticker := time.NewTicker(time.Duration(request.Interval))
	defer ticker.Stop()
	trafficManager := boxService.clashServer.(*clashapi.Server).TrafficManager()
	var (
		connections    = make(map[uuid.UUID]*Connection)
		outConnections []*Connection
	)
	for {
		outConnections = outConnections[:0]
		for _, connection := range trafficManager.Connections() {
			outConnections = append(outConnections, newConnection(connections, connection, false))
		}
		for _, connection := range trafficManager.ClosedConnections() {
			outConnections = append(outConnections, newConnection(connections, connection, true))
		}
		err := server.Send(&Connections{Connections: outConnections})
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
	}
}

func newConnection(connections map[uuid.UUID]*Connection, metadata trafficontrol.TrackerMetadata, isClosed bool) *Connection {
	if oldConnection, loaded := connections[metadata.ID]; loaded {
		if isClosed {
			if oldConnection.ClosedAt == 0 {
				oldConnection.Uplink = 0
				oldConnection.Downlink = 0
				oldConnection.ClosedAt = metadata.ClosedAt.UnixMilli()
			}
			return oldConnection
		}
		lastUplink := oldConnection.UplinkTotal
		lastDownlink := oldConnection.DownlinkTotal
		uplinkTotal := metadata.Upload.Load()
		downlinkTotal := metadata.Download.Load()
		oldConnection.Uplink = uplinkTotal - lastUplink
		oldConnection.Downlink = downlinkTotal - lastDownlink
		oldConnection.UplinkTotal = uplinkTotal
		oldConnection.DownlinkTotal = downlinkTotal
		return oldConnection
	}
	var rule string
	if metadata.Rule != nil {
		rule = metadata.Rule.String()
	}
	uplinkTotal := metadata.Upload.Load()
	downlinkTotal := metadata.Download.Load()
	uplink := uplinkTotal
	downlink := downlinkTotal
	var closedAt int64
	if !metadata.ClosedAt.IsZero() {
		closedAt = metadata.ClosedAt.UnixMilli()
		uplink = 0
		downlink = 0
	}
	connection := &Connection{
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
		ClosedAt:      closedAt,
		Uplink:        uplink,
		Downlink:      downlink,
		UplinkTotal:   uplinkTotal,
		DownlinkTotal: downlinkTotal,
		Rule:          rule,
		Outbound:      metadata.Outbound,
		OutboundType:  metadata.OutboundType,
		ChainList:     metadata.Chain,
	}
	connections[metadata.ID] = connection
	return connection
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
	conntrack.Close()
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
				Message:       it.Message(),
				Impending:     it.Impending(),
				MigrationLink: it.MigrationLink,
			}
		}),
	}, nil
}

func (s *StartedService) SubscribeHelperEvents(empty *emptypb.Empty, server grpc.ServerStreamingServer[HelperRequest]) error {
	return os.ErrInvalid
}

func (s *StartedService) SendHelperResponse(ctx context.Context, response *HelperResponse) (*emptypb.Empty, error) {
	return nil, os.ErrInvalid
}

func (s *StartedService) mustEmbedUnimplementedStartedServiceServer() {
}

func (s *StartedService) WriteMessage(level log.Level, message string) {
	item := &log.Entry{Level: level, Message: message}
	s.logSubscriber.Emit(item)
	s.logAccess.Lock()
	s.logLines.PushBack(item)
	if s.logLines.Len() > s.logMaxLines {
		s.logLines.Remove(s.logLines.Front())
	}
	s.logAccess.Unlock()
	if s.debug {
		s.handler.WriteDebugMessage(message)
	}
}

func (s *StartedService) Instance() *Instance {
	s.serviceAccess.RLock()
	defer s.serviceAccess.RUnlock()
	return s.instance
}
