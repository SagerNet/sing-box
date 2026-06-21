package libbox

import (
	"context"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/sagernet/sing-box/daemon"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type CommandClient struct {
	handler           CommandClientHandler
	grpcConn          *grpc.ClientConn
	grpcClient        daemon.StartedServiceClient
	grpcManagedClient daemon.ManagedServiceClient
	options           CommandClientOptions
	remote            *remoteConnection
	ctx               context.Context
	cancel            context.CancelFunc
	clientMutex       sync.RWMutex
	standalone        bool
}

type CommandClientOptions struct {
	commands       []int32
	StatusInterval int64
}

func (o *CommandClientOptions) AddCommand(command int32) {
	o.commands = append(o.commands, command)
}

type CommandClientHandler interface {
	Connected()
	Disconnected(message string)
	SetDefaultLogLevel(level int32)
	ClearLogs()
	WriteLogs(messageList LogIterator)
	WriteStatus(message *StatusMessage)
	WriteGroups(message OutboundGroupIterator)
	WriteOutbounds(message OutboundGroupItemIterator)
	InitializeClashMode(modeList StringIterator, currentMode string)
	UpdateClashMode(newMode string)
	WriteConnectionEvents(events *ConnectionEvents)
}

type LogEntry struct {
	Level   int32
	Message string
}

type LogIterator interface {
	Len() int32
	HasNext() bool
	Next() *LogEntry
}

type XPCDialer interface {
	DialXPC() (int32, error)
}

var sXPCDialer XPCDialer

func SetXPCDialer(dialer XPCDialer) {
	sXPCDialer = dialer
}

func NewStandaloneCommandClient() *CommandClient {
	return &CommandClient{standalone: true}
}

func NewCommandClient(handler CommandClientHandler, options *CommandClientOptions) *CommandClient {
	return &CommandClient{
		handler: handler,
		options: common.PtrValueOrDefault(options),
	}
}

func unaryClientAuthInterceptor(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	if sCommandServerSecret != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-command-secret", sCommandServerSecret)
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}

func streamClientAuthInterceptor(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	if sCommandServerSecret != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-command-secret", sCommandServerSecret)
	}
	return streamer(ctx, desc, cc, method, opts...)
}

const (
	commandClientDialAttempts  = 10
	commandClientDialBaseDelay = 100 * time.Millisecond
	commandClientDialStepDelay = 50 * time.Millisecond
)

func commandClientDialDelay(attempt int) time.Duration {
	return commandClientDialBaseDelay + time.Duration(attempt)*commandClientDialStepDelay
}

func dialTarget() (string, func(context.Context, string) (net.Conn, error)) {
	if sXPCDialer != nil {
		return "passthrough:///xpc", func(ctx context.Context, _ string) (net.Conn, error) {
			fileDescriptor, err := sXPCDialer.DialXPC()
			if err != nil {
				return nil, E.Cause(err, "dial xpc")
			}
			return networkConnectionFromFileDescriptor(fileDescriptor)
		}
	}
	if sCommandServerListenPort == 0 {
		socketPath := filepath.Join(sBasePath, "command.sock")
		return "passthrough:///command-socket", func(ctx context.Context, _ string) (net.Conn, error) {
			var networkDialer net.Dialer
			return networkDialer.DialContext(ctx, "unix", socketPath)
		}
	}
	return net.JoinHostPort("127.0.0.1", strconv.Itoa(int(sCommandServerListenPort))), nil
}

func networkConnectionFromFileDescriptor(fileDescriptor int32) (net.Conn, error) {
	file := os.NewFile(uintptr(fileDescriptor), "xpc-command-socket")
	if file == nil {
		return nil, E.New("invalid file descriptor")
	}
	networkConnection, err := net.FileConn(file)
	if err != nil {
		file.Close()
		return nil, E.Cause(err, "create connection from fd")
	}
	file.Close()
	return networkConnection, nil
}

func localDialOptions(contextDialer func(context.Context, string) (net.Conn, error)) []grpc.DialOption {
	options := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(unaryClientAuthInterceptor),
		grpc.WithStreamInterceptor(streamClientAuthInterceptor),
	}
	if contextDialer != nil {
		options = append(options, grpc.WithContextDialer(contextDialer))
	}
	return options
}

// establishConnection dials the command server the client is bound to: the
// local command server (over socket/XPC) or a remote API service.
func (c *CommandClient) establishConnection() (*grpc.ClientConn, daemon.StartedServiceClient, error) {
	if c.remote != nil {
		return c.dialRemote()
	}
	target, contextDialer := dialTarget()
	return c.dialWithRetry(target, localDialOptions(contextDialer), true)
}

// dialWithRetry connects to the local command server. The retry loop exists to
// wait out the server starting up: WaitForReady keeps the probe redialing and
// the loop reissues it with a growing delay, so a freshly launched extension is
// picked up without surfacing a transient "unavailable" to the UI.
func (c *CommandClient) dialWithRetry(target string, dialOptions []grpc.DialOption, retryDial bool) (*grpc.ClientConn, daemon.StartedServiceClient, error) {
	var connection *grpc.ClientConn
	var client daemon.StartedServiceClient
	var lastError error

	for attempt := range commandClientDialAttempts {
		if connection == nil {
			var err error
			connection, err = grpc.NewClient(target, dialOptions...)
			if err != nil {
				lastError = err
				if !retryDial {
					return nil, nil, E.Cause(err, "create command client")
				}
				time.Sleep(commandClientDialDelay(attempt))
				continue
			}
			client = daemon.NewStartedServiceClient(connection)
		}
		ctx, cancel := context.WithTimeout(context.Background(), commandClientDialDelay(attempt))
		_, err := client.GetStartedAt(ctx, &emptypb.Empty{}, grpc.WaitForReady(true))
		cancel()
		if err == nil {
			return connection, client, nil
		}
		lastError = err
	}

	if connection != nil {
		connection.Close()
	}
	return nil, nil, E.Cause(lastError, "probe command server")
}

func (c *CommandClient) dialRemote() (*grpc.ClientConn, daemon.StartedServiceClient, error) {
	connection, err := grpc.NewClient(c.remote.target, c.remote.dialOptions...)
	if err != nil {
		return nil, nil, E.Cause(err, "create remote command client")
	}
	client := daemon.NewStartedServiceClient(connection)
	ctx, cancel := context.WithTimeout(context.Background(), commandClientRemoteProbeTimeout)
	defer cancel()
	_, err = client.GetStartedAt(ctx, &emptypb.Empty{})
	if err != nil {
		connection.Close()
		return nil, nil, E.Cause(err, "connect to remote server")
	}
	return connection, client, nil
}

func (c *CommandClient) Connect() error {
	c.clientMutex.Lock()
	common.Close(common.PtrOrNil(c.grpcConn))

	connection, client, err := c.establishConnection()
	if err != nil {
		c.clientMutex.Unlock()
		return err
	}
	c.grpcConn = connection
	c.grpcClient = client
	c.grpcManagedClient = daemon.NewManagedServiceClient(connection)
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.clientMutex.Unlock()

	c.handler.Connected()
	return c.dispatchCommands()
}

func (c *CommandClient) ConnectWithFD(fd int32) error {
	c.clientMutex.Lock()
	common.Close(common.PtrOrNil(c.grpcConn))

	networkConnection, err := networkConnectionFromFileDescriptor(fd)
	if err != nil {
		c.clientMutex.Unlock()
		return err
	}
	connection, client, err := c.dialWithRetry("passthrough:///xpc", localDialOptions(func(ctx context.Context, _ string) (net.Conn, error) {
		return networkConnection, nil
	}), false)
	if err != nil {
		networkConnection.Close()
		c.clientMutex.Unlock()
		return err
	}
	c.grpcConn = connection
	c.grpcClient = client
	c.grpcManagedClient = daemon.NewManagedServiceClient(connection)
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.clientMutex.Unlock()

	c.handler.Connected()
	return c.dispatchCommands()
}

func (c *CommandClient) dispatchCommands() error {
	for _, command := range c.options.commands {
		switch command {
		case CommandLog:
			go c.handleLogStream()
		case CommandStatus:
			go c.handleStatusStream()
		case CommandGroup:
			go c.handleGroupStream()
		case CommandClashMode:
			go c.handleClashModeStream()
		case CommandConnections:
			go c.handleConnectionsStream()
		case CommandOutbounds:
			go c.handleOutboundsStream()
		default:
			return E.New("unknown command: ", command)
		}
	}
	return nil
}

func (c *CommandClient) Disconnect() error {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
	if c.cancel != nil {
		c.cancel()
	}
	return common.Close(common.PtrOrNil(c.grpcConn))
}

func (c *CommandClient) getClientForCall() (daemon.StartedServiceClient, context.Context, error) {
	c.clientMutex.RLock()
	if c.grpcClient != nil {
		defer c.clientMutex.RUnlock()
		return c.grpcClient, c.ctx, nil
	}
	c.clientMutex.RUnlock()

	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	if c.grpcClient != nil {
		return c.grpcClient, c.ctx, nil
	}

	connection, client, err := c.establishConnection()
	if err != nil {
		return nil, nil, E.Cause(err, "get command client")
	}
	c.grpcConn = connection
	c.grpcClient = client
	c.grpcManagedClient = daemon.NewManagedServiceClient(connection)
	if c.ctx == nil {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	}
	return c.grpcClient, c.ctx, nil
}

func (c *CommandClient) closeConnection() {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
	if c.grpcConn != nil {
		c.grpcConn.Close()
		c.grpcConn = nil
		c.grpcClient = nil
		c.grpcManagedClient = nil
	}
}

func callWithResult[T any](c *CommandClient, call func(ctx context.Context, client daemon.StartedServiceClient) (T, error)) (T, error) {
	client, ctx, err := c.getClientForCall()
	if err != nil {
		var zero T
		return zero, err
	}
	if c.standalone {
		defer c.closeConnection()
	}
	return call(ctx, client)
}

func callManagedWithResult[T any](c *CommandClient, call func(ctx context.Context, client daemon.ManagedServiceClient) (T, error)) (T, error) {
	_, ctx, err := c.getClientForCall()
	if err != nil {
		var zero T
		return zero, err
	}
	if c.standalone {
		defer c.closeConnection()
	}
	c.clientMutex.RLock()
	client := c.grpcManagedClient
	c.clientMutex.RUnlock()
	if client == nil {
		var zero T
		return zero, os.ErrClosed
	}
	return call(ctx, client)
}

func (c *CommandClient) getStreamContext() (daemon.StartedServiceClient, context.Context) {
	c.clientMutex.RLock()
	defer c.clientMutex.RUnlock()
	return c.grpcClient, c.ctx
}

func (c *CommandClient) handleLogStream() {
	client, ctx := c.getStreamContext()
	stream, err := client.SubscribeLog(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(E.Cause(err, "subscribe log").Error())
		return
	}
	defaultLogLevel, err := client.GetDefaultLogLevel(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(E.Cause(err, "get default log level").Error())
		return
	}
	c.handler.SetDefaultLogLevel(int32(defaultLogLevel.Level))
	for {
		logMessage, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(E.Cause(err, "log stream recv").Error())
			return
		}
		if logMessage.Reset_ {
			c.handler.ClearLogs()
		}
		var messages []*LogEntry
		for _, msg := range logMessage.Messages {
			messages = append(messages, &LogEntry{
				Level:   int32(msg.Level),
				Message: msg.Message,
			})
		}
		c.handler.WriteLogs(newIterator(messages))
	}
}

func (c *CommandClient) handleStatusStream() {
	client, ctx := c.getStreamContext()
	interval := c.options.StatusInterval

	stream, err := client.SubscribeStatus(ctx, &daemon.SubscribeStatusRequest{
		Interval: interval,
	})
	if err != nil {
		c.handler.Disconnected(E.Cause(err, "subscribe status").Error())
		return
	}

	for {
		status, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(E.Cause(err, "status stream recv").Error())
			return
		}
		c.handler.WriteStatus(statusMessageFromGRPC(status))
	}
}

func (c *CommandClient) handleGroupStream() {
	client, ctx := c.getStreamContext()

	stream, err := client.SubscribeGroups(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(E.Cause(err, "subscribe groups").Error())
		return
	}

	for {
		groups, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(E.Cause(err, "groups stream recv").Error())
			return
		}
		c.handler.WriteGroups(outboundGroupIteratorFromGRPC(groups))
	}
}

func (c *CommandClient) handleClashModeStream() {
	client, ctx := c.getStreamContext()

	modeStatus, err := client.GetClashModeStatus(ctx, &emptypb.Empty{})
	if err != nil {
		if status.Code(err) != codes.NotFound {
			c.handler.Disconnected(E.Cause(err, "get clash mode status").Error())
			return
		}
		modeStatus = &daemon.ClashModeStatus{}
	}

	if sFixAndroidStack {
		go c.handler.InitializeClashMode(newIterator(modeStatus.ModeList), modeStatus.CurrentMode)
	} else {
		c.handler.InitializeClashMode(newIterator(modeStatus.ModeList), modeStatus.CurrentMode)
	}

	if len(modeStatus.ModeList) == 0 {
		return
	}

	stream, err := client.SubscribeClashMode(ctx, &emptypb.Empty{})
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return
		}
		c.handler.Disconnected(E.Cause(err, "subscribe clash mode").Error())
		return
	}

	for {
		mode, err := stream.Recv()
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return
			}
			c.handler.Disconnected(E.Cause(err, "clash mode stream recv").Error())
			return
		}
		c.handler.UpdateClashMode(mode.Mode)
	}
}

func (c *CommandClient) handleConnectionsStream() {
	client, ctx := c.getStreamContext()
	interval := c.options.StatusInterval

	stream, err := client.SubscribeConnections(ctx, &daemon.SubscribeConnectionsRequest{
		Interval: interval,
	})
	if err != nil {
		c.handler.Disconnected(E.Cause(err, "subscribe connections").Error())
		return
	}

	for {
		events, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(E.Cause(err, "connections stream recv").Error())
			return
		}
		libboxEvents := connectionEventsFromGRPC(events)
		c.handler.WriteConnectionEvents(libboxEvents)
	}
}

func (c *CommandClient) handleOutboundsStream() {
	client, ctx := c.getStreamContext()

	stream, err := client.SubscribeOutbounds(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(E.Cause(err, "subscribe outbounds").Error())
		return
	}

	for {
		list, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(E.Cause(err, "outbounds stream recv").Error())
			return
		}
		c.handler.WriteOutbounds(outboundGroupItemListFromGRPC(list))
	}
}

func (c *CommandClient) SelectOutbound(groupTag string, outboundTag string) error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SelectOutbound(ctx, &daemon.SelectOutboundRequest{
			GroupTag:    groupTag,
			OutboundTag: outboundTag,
		})
	})
	if err != nil {
		return E.Cause(err, "select outbound")
	}
	return nil
}

func (c *CommandClient) URLTest(groupTag string) error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.URLTest(ctx, &daemon.URLTestRequest{
			OutboundTag: groupTag,
		})
	})
	if err != nil {
		return E.Cause(err, "url test")
	}
	return nil
}

func (c *CommandClient) SetClashMode(newMode string) error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SetClashMode(ctx, &daemon.ClashMode{
			Mode: newMode,
		})
	})
	if err != nil {
		return E.Cause(err, "set clash mode")
	}
	return nil
}

func (c *CommandClient) CloseConnection(connId string) error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.CloseConnection(ctx, &daemon.CloseConnectionRequest{
			Id: connId,
		})
	})
	if err != nil {
		return E.Cause(err, "close connection")
	}
	return nil
}

func (c *CommandClient) CloseConnections() error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.CloseAllConnections(ctx, &emptypb.Empty{})
	})
	if err != nil {
		return E.Cause(err, "close all connections")
	}
	return nil
}

func (c *CommandClient) ServiceReload() error {
	_, err := callManagedWithResult(c, func(ctx context.Context, client daemon.ManagedServiceClient) (*emptypb.Empty, error) {
		return client.ReloadService(ctx, &emptypb.Empty{})
	})
	if err != nil {
		return E.Cause(err, "reload service")
	}
	return nil
}

func (c *CommandClient) ServiceClose() error {
	_, err := callManagedWithResult(c, func(ctx context.Context, client daemon.ManagedServiceClient) (*emptypb.Empty, error) {
		return client.StopService(ctx, &emptypb.Empty{})
	})
	if err != nil {
		return E.Cause(err, "stop service")
	}
	return nil
}

func (c *CommandClient) ClearLogs() error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.ClearLogs(ctx, &emptypb.Empty{})
	})
	if err != nil {
		return E.Cause(err, "clear logs")
	}
	return nil
}

func (c *CommandClient) GetSystemProxyStatus() (*SystemProxyStatus, error) {
	return callManagedWithResult(c, func(ctx context.Context, client daemon.ManagedServiceClient) (*SystemProxyStatus, error) {
		status, err := client.GetSystemProxyStatus(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, E.Cause(err, "get system proxy status")
		}
		return systemProxyStatusFromGRPC(status), nil
	})
}

func (c *CommandClient) SetSystemProxyEnabled(isEnabled bool) error {
	_, err := callManagedWithResult(c, func(ctx context.Context, client daemon.ManagedServiceClient) (*emptypb.Empty, error) {
		return client.SetSystemProxyEnabled(ctx, &daemon.SetSystemProxyEnabledRequest{
			Enabled: isEnabled,
		})
	})
	if err != nil {
		return E.Cause(err, "set system proxy enabled")
	}
	return nil
}

func (c *CommandClient) TriggerGoCrash() error {
	_, err := callManagedWithResult(c, func(ctx context.Context, client daemon.ManagedServiceClient) (*emptypb.Empty, error) {
		return client.TriggerDebugCrash(ctx, &daemon.DebugCrashRequest{
			Type: daemon.DebugCrashRequest_GO,
		})
	})
	if err != nil {
		return E.Cause(err, "trigger debug crash")
	}
	return nil
}

func (c *CommandClient) TriggerNativeCrash() error {
	_, err := callManagedWithResult(c, func(ctx context.Context, client daemon.ManagedServiceClient) (*emptypb.Empty, error) {
		return client.TriggerDebugCrash(ctx, &daemon.DebugCrashRequest{
			Type: daemon.DebugCrashRequest_NATIVE,
		})
	})
	if err != nil {
		return E.Cause(err, "trigger native crash")
	}
	return nil
}

func (c *CommandClient) TriggerOOMReport() error {
	_, err := callManagedWithResult(c, func(ctx context.Context, client daemon.ManagedServiceClient) (*emptypb.Empty, error) {
		return client.TriggerOOMReport(ctx, &emptypb.Empty{})
	})
	if err != nil {
		return E.Cause(err, "trigger oom report")
	}
	return nil
}

func (c *CommandClient) GetDeprecatedNotes() (DeprecatedNoteIterator, error) {
	return callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (DeprecatedNoteIterator, error) {
		warnings, err := client.GetDeprecatedWarnings(ctx, &emptypb.Empty{})
		if err != nil {
			return nil, E.Cause(err, "get deprecated warnings")
		}
		var notes []*DeprecatedNote
		for _, warning := range warnings.Warnings {
			notes = append(notes, &DeprecatedNote{
				Description:       warning.Description,
				DeprecatedVersion: warning.DeprecatedVersion,
				ScheduledVersion:  warning.ScheduledVersion,
				MigrationLink:     warning.MigrationLink,
			})
		}
		return newIterator(notes), nil
	})
}

func (c *CommandClient) GetStartedAt() (int64, error) {
	return callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (int64, error) {
		startedAt, err := client.GetStartedAt(ctx, &emptypb.Empty{})
		if err != nil {
			return 0, E.Cause(err, "get started at")
		}
		return startedAt.StartedAt, nil
	})
}

func (c *CommandClient) GetAPIVersion() (int32, error) {
	return callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (int32, error) {
		version, err := client.GetVersion(ctx, &emptypb.Empty{})
		if err != nil {
			return 0, E.Cause(err, "get version")
		}
		return version.ApiVersion, nil
	})
}

func (c *CommandClient) SetGroupExpand(groupTag string, isExpand bool) error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SetGroupExpand(ctx, &daemon.SetGroupExpandRequest{
			GroupTag: groupTag,
			IsExpand: isExpand,
		})
	})
	if err != nil {
		return E.Cause(err, "set group expand")
	}
	return nil
}

func (c *CommandClient) StartNetworkQualityTest(configURL string, outboundTag string, serial bool, maxRuntimeSeconds int32, http3 bool, handler NetworkQualityTestHandler) (*NetworkQualityTestSession, error) {
	client, parentCtx, err := c.getClientForCall()
	if err != nil {
		return nil, E.Cause(err, "start network quality test")
	}

	streamCtx, cancel := context.WithCancel(parentCtx)
	session := &NetworkQualityTestSession{
		streamSession: streamSession{
			ctx:       streamCtx,
			cancel:    cancel,
			closeDone: make(chan struct{}),
		},
	}

	failStart := func(cause error, message string) (*NetworkQualityTestSession, error) {
		cancel()
		if c.standalone {
			c.closeConnection()
		}
		return nil, E.Cause(cause, message)
	}

	stream, err := client.StartNetworkQualityTest(streamCtx, &daemon.NetworkQualityTestRequest{
		ConfigURL:         configURL,
		OutboundTag:       outboundTag,
		Serial:            serial,
		MaxRuntimeSeconds: maxRuntimeSeconds,
		Http3:             http3,
	})
	if err != nil {
		return failStart(err, "start network quality test")
	}

	standalone := c.standalone
	go func() {
		defer func() {
			close(session.closeDone)
			if standalone {
				c.closeConnection()
			}
		}()
		for {
			event, recvErr := stream.Recv()
			if recvErr != nil {
				if session.ctx.Err() != nil {
					return
				}
				handler.OnError(E.Cause(recvErr, "network quality test recv").Error())
				return
			}
			if event.IsFinal {
				if event.Error != "" {
					handler.OnError(event.Error)
				} else {
					handler.OnResult(&NetworkQualityResult{
						DownloadCapacity:         event.DownloadCapacity,
						UploadCapacity:           event.UploadCapacity,
						DownloadRPM:              event.DownloadRPM,
						UploadRPM:                event.UploadRPM,
						IdleLatencyMs:            event.IdleLatencyMs,
						DownloadCapacityAccuracy: event.DownloadCapacityAccuracy,
						UploadCapacityAccuracy:   event.UploadCapacityAccuracy,
						DownloadRPMAccuracy:      event.DownloadRPMAccuracy,
						UploadRPMAccuracy:        event.UploadRPMAccuracy,
					})
				}
				return
			}
			handler.OnProgress(networkQualityProgressFromGRPC(event))
		}
	}()

	return session, nil
}

func (c *CommandClient) StartSTUNTest(server string, outboundTag string, handler STUNTestHandler) (*STUNTestSession, error) {
	client, parentCtx, err := c.getClientForCall()
	if err != nil {
		return nil, E.Cause(err, "start stun test")
	}

	streamCtx, cancel := context.WithCancel(parentCtx)
	session := &STUNTestSession{
		streamSession: streamSession{
			ctx:       streamCtx,
			cancel:    cancel,
			closeDone: make(chan struct{}),
		},
	}

	failStart := func(cause error, message string) (*STUNTestSession, error) {
		cancel()
		if c.standalone {
			c.closeConnection()
		}
		return nil, E.Cause(cause, message)
	}

	stream, err := client.StartSTUNTest(streamCtx, &daemon.STUNTestRequest{
		Server:      server,
		OutboundTag: outboundTag,
	})
	if err != nil {
		return failStart(err, "start stun test")
	}

	standalone := c.standalone
	go func() {
		defer func() {
			close(session.closeDone)
			if standalone {
				c.closeConnection()
			}
		}()
		for {
			event, recvErr := stream.Recv()
			if recvErr != nil {
				if session.ctx.Err() != nil {
					return
				}
				handler.OnError(E.Cause(recvErr, "stun test recv").Error())
				return
			}
			if event.IsFinal {
				if event.Error != "" {
					handler.OnError(event.Error)
				} else {
					handler.OnResult(&STUNTestResult{
						ExternalAddr:     event.ExternalAddr,
						LatencyMs:        event.LatencyMs,
						NATMapping:       event.NatMapping,
						NATFiltering:     event.NatFiltering,
						NATTypeSupported: event.NatTypeSupported,
					})
				}
				return
			}
			handler.OnProgress(stunTestProgressFromGRPC(event))
		}
	}()

	return session, nil
}

func (c *CommandClient) SubscribeTailscaleStatus(handler TailscaleStatusHandler) (*TailscaleStatusSubscription, error) {
	client, parentCtx, err := c.getClientForCall()
	if err != nil {
		return nil, E.Cause(err, "subscribe tailscale status")
	}

	streamCtx, cancel := context.WithCancel(parentCtx)
	session := &TailscaleStatusSubscription{
		streamSession: streamSession{
			ctx:       streamCtx,
			cancel:    cancel,
			closeDone: make(chan struct{}),
		},
	}

	failStart := func(cause error, message string) (*TailscaleStatusSubscription, error) {
		cancel()
		if c.standalone {
			c.closeConnection()
		}
		return nil, E.Cause(cause, message)
	}

	stream, err := client.SubscribeTailscaleStatus(streamCtx, &emptypb.Empty{})
	if err != nil {
		return failStart(err, "subscribe tailscale status")
	}

	standalone := c.standalone
	go func() {
		defer func() {
			close(session.closeDone)
			if standalone {
				c.closeConnection()
			}
		}()
		for {
			event, recvErr := stream.Recv()
			if recvErr != nil {
				if session.ctx.Err() != nil {
					return
				}
				if status.Code(recvErr) == codes.NotFound || status.Code(recvErr) == codes.Unavailable {
					return
				}
				handler.OnError(E.Cause(recvErr, "tailscale status recv").Error())
				return
			}
			handler.OnStatusUpdate(tailscaleStatusUpdateFromGRPC(event))
		}
	}()

	return session, nil
}

func (c *CommandClient) SubscribeUSBIPServerStatus(handler USBIPServerStatusHandler) (*USBIPServerStatusSubscription, error) {
	client, parentCtx, err := c.getClientForCall()
	if err != nil {
		return nil, E.Cause(err, "subscribe usbip server status")
	}

	streamCtx, cancel := context.WithCancel(parentCtx)
	session := &USBIPServerStatusSubscription{
		streamSession: streamSession{
			ctx:       streamCtx,
			cancel:    cancel,
			closeDone: make(chan struct{}),
		},
	}

	failStart := func(cause error, message string) (*USBIPServerStatusSubscription, error) {
		cancel()
		if c.standalone {
			c.closeConnection()
		}
		return nil, E.Cause(cause, message)
	}

	stream, err := client.SubscribeUSBIPServerStatus(streamCtx, &emptypb.Empty{})
	if err != nil {
		return failStart(err, "subscribe usbip server status")
	}

	standalone := c.standalone
	go func() {
		defer func() {
			close(session.closeDone)
			if standalone {
				c.closeConnection()
			}
		}()
		for {
			event, recvErr := stream.Recv()
			if recvErr != nil {
				if session.ctx.Err() != nil {
					return
				}
				if status.Code(recvErr) == codes.NotFound || status.Code(recvErr) == codes.Unavailable {
					return
				}
				handler.OnError(E.Cause(recvErr, "usbip server status recv").Error())
				return
			}
			handler.OnStatusUpdate(usbipServerStatusUpdateFromGRPC(event))
		}
	}()

	return session, nil
}

func (c *CommandClient) SetTailscaleExitNode(endpointTag string, stableID string) error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SetTailscaleExitNode(ctx, &daemon.SetTailscaleExitNodeRequest{
			EndpointTag: endpointTag,
			StableID:    stableID,
		})
	})
	if err != nil {
		return E.Cause(err, "set tailscale exit node")
	}
	return nil
}

func (c *CommandClient) TailscaleLogout(endpointTag string) error {
	_, err := callWithResult(c, func(ctx context.Context, client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.TailscaleLogout(ctx, &daemon.TailscaleLogoutRequest{
			EndpointTag: endpointTag,
		})
	})
	if err != nil {
		return E.Cause(err, "tailscale logout")
	}
	return nil
}

func (c *CommandClient) StartTailscalePing(endpointTag string, peerIP string, handler TailscalePingHandler) (*TailscalePingSession, error) {
	client, parentCtx, err := c.getClientForCall()
	if err != nil {
		return nil, E.Cause(err, "start tailscale ping")
	}

	streamCtx, cancel := context.WithCancel(parentCtx)
	session := &TailscalePingSession{
		streamSession: streamSession{
			ctx:       streamCtx,
			cancel:    cancel,
			closeDone: make(chan struct{}),
		},
	}

	failStart := func(cause error, message string) (*TailscalePingSession, error) {
		cancel()
		if c.standalone {
			c.closeConnection()
		}
		return nil, E.Cause(cause, message)
	}

	stream, err := client.StartTailscalePing(streamCtx, &daemon.TailscalePingRequest{
		EndpointTag: endpointTag,
		PeerIP:      peerIP,
	})
	if err != nil {
		return failStart(err, "start tailscale ping")
	}

	standalone := c.standalone
	go func() {
		defer func() {
			close(session.closeDone)
			if standalone {
				c.closeConnection()
			}
		}()
		for {
			event, recvErr := stream.Recv()
			if recvErr != nil {
				if session.ctx.Err() != nil {
					return
				}
				handler.OnError(E.Cause(recvErr, "tailscale ping recv").Error())
				return
			}
			handler.OnPingResult(tailscalePingResultFromGRPC(event))
		}
	}()

	return session, nil
}

func (c *CommandClient) StartTailscaleSSHSession(opts *TailscaleSSHOptions, handler TailscaleSSHHandler) (*TailscaleSSHSession, error) {
	client, parentCtx, err := c.getClientForCall()
	if err != nil {
		return nil, E.Cause(err, "start tailscale ssh session")
	}

	streamCtx, cancel := context.WithCancel(parentCtx)
	failStart := func(cause error, message string) (*TailscaleSSHSession, error) {
		cancel()
		if c.standalone {
			c.closeConnection()
		}
		return nil, E.Cause(cause, message)
	}

	stream, err := client.StartTailscaleSSHSession(streamCtx)
	if err != nil {
		return failStart(err, "start tailscale ssh session")
	}

	sendErr := stream.Send(&daemon.TailscaleSSHClientMessage{
		Message: &daemon.TailscaleSSHClientMessage_Start{Start: &daemon.TailscaleSSHStart{
			EndpointTag:  opts.EndpointTag,
			PeerAddress:  opts.PeerAddress,
			Username:     opts.Username,
			TerminalType: opts.TerminalType,
			Columns:      opts.Columns,
			Rows:         opts.Rows,
			WidthPixels:  opts.WidthPixels,
			HeightPixels: opts.HeightPixels,
			HostKeys:     iteratorToArray[string](opts.HostKeys),
			ForwardAgent: opts.ForwardAgent,
		}},
	})
	if sendErr != nil {
		return failStart(sendErr, "send tailscale ssh start")
	}

	session := &TailscaleSSHSession{
		stream:    stream,
		inputCh:   make(chan []byte, 8),
		resizeCh:  make(chan tailscaleSSHResize, 1),
		ctx:       streamCtx,
		cancel:    cancel,
		closeDone: make(chan struct{}),
	}

	session.wg.Add(1)
	go func() {
		defer session.wg.Done()
		for {
			select {
			case <-streamCtx.Done():
				return
			case data := <-session.inputCh:
				sendErr := stream.Send(&daemon.TailscaleSSHClientMessage{
					Message: &daemon.TailscaleSSHClientMessage_Input{Input: &daemon.TailscaleSSHInput{Data: data}},
				})
				if sendErr != nil {
					cancel()
					return
				}
			case resize := <-session.resizeCh:
				sendErr := stream.Send(&daemon.TailscaleSSHClientMessage{
					Message: &daemon.TailscaleSSHClientMessage_Resize{Resize: &daemon.TailscaleSSHResize{
						Columns:      resize.columns,
						Rows:         resize.rows,
						WidthPixels:  resize.widthPixels,
						HeightPixels: resize.heightPixels,
					}},
				})
				if sendErr != nil {
					cancel()
					return
				}
			}
		}
	}()

	session.wg.Add(1)
	go func() {
		defer session.wg.Done()
		for {
			msg, recvErr := stream.Recv()
			if recvErr == io.EOF {
				cancel()
				return
			}
			if recvErr != nil {
				handler.OnError(E.Cause(recvErr, "tailscale ssh recv").Error())
				cancel()
				return
			}
			switch payload := msg.GetMessage().(type) {
			case *daemon.TailscaleSSHServerMessage_AuthBanner:
				handler.OnAuthBanner(payload.AuthBanner.Message)
			case *daemon.TailscaleSSHServerMessage_Ready:
				handler.OnReady()
			case *daemon.TailscaleSSHServerMessage_Output:
				handler.OnOutput(payload.Output.Data)
			case *daemon.TailscaleSSHServerMessage_Exit:
				handler.OnExit(payload.Exit.ExitCode, payload.Exit.Signal, payload.Exit.ErrorMessage)
				cancel()
				return
			case *daemon.TailscaleSSHServerMessage_Error:
				handler.OnError(payload.Error.Message)
			}
		}
	}()

	standalone := c.standalone
	go func() {
		session.wg.Wait()
		close(session.closeDone)
		if standalone {
			c.closeConnection()
		}
	}()

	return session, nil
}

func (c *CommandClient) ProvideUSBDevices(handler USBProviderHandler) (*USBProviderSession, error) {
	client, parentCtx, err := c.getClientForCall()
	if err != nil {
		return nil, E.Cause(err, "provide usb devices")
	}

	streamCtx, cancel := context.WithCancel(parentCtx)
	stream, err := client.ProvideUSBDevices(streamCtx)
	if err != nil {
		cancel()
		if c.standalone {
			c.closeConnection()
		}
		return nil, E.Cause(err, "provide usb devices")
	}

	session := &USBProviderSession{
		stream:    stream,
		ctx:       streamCtx,
		cancel:    cancel,
		closeDone: make(chan struct{}),
	}

	standalone := c.standalone
	go func() {
		defer close(session.closeDone)
		for {
			message, recvErr := stream.Recv()
			if recvErr == io.EOF {
				cancel()
				break
			}
			if recvErr != nil {
				handler.OnError("", E.Cause(recvErr, "usb provider recv").Error())
				cancel()
				break
			}
			switch payload := message.GetMessage().(type) {
			case *daemon.USBServerMessage_Ready:
				handler.OnReady(payload.Ready.GetDeviceId(), payload.Ready.GetBusId())
			case *daemon.USBServerMessage_UrbRequest:
				handler.OnURBRequest(usbURBRequestFromGRPC(payload.UrbRequest))
			case *daemon.USBServerMessage_Abort:
				handler.OnAbort(payload.Abort.GetDeviceId(), int32(payload.Abort.GetEndpoint()))
			case *daemon.USBServerMessage_Error:
				handler.OnError(payload.Error.GetDeviceId(), payload.Error.GetMessage())
			}
		}
		if standalone {
			c.closeConnection()
		}
	}()

	return session, nil
}
