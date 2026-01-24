package libbox

import (
	"context"
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
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/emptypb"
)

type CommandClient struct {
	handler     CommandClientHandler
	grpcConn    *grpc.ClientConn
	grpcClient  daemon.StartedServiceClient
	options     CommandClientOptions
	ctx         context.Context
	cancel      context.CancelFunc
	clientMutex sync.RWMutex
	standalone  bool
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
				return nil, err
			}
			return networkConnectionFromFileDescriptor(fileDescriptor)
		}
	}
	if sCommandServerListenPort == 0 {
		return "unix://" + filepath.Join(sBasePath, "command.sock"), nil
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

func (c *CommandClient) dialWithRetry(target string, contextDialer func(context.Context, string) (net.Conn, error), retryDial bool) (*grpc.ClientConn, daemon.StartedServiceClient, error) {
	var connection *grpc.ClientConn
	var client daemon.StartedServiceClient
	var lastError error

	for attempt := 0; attempt < commandClientDialAttempts; attempt++ {
		if connection == nil {
			options := []grpc.DialOption{
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithUnaryInterceptor(unaryClientAuthInterceptor),
				grpc.WithStreamInterceptor(streamClientAuthInterceptor),
			}
			if contextDialer != nil {
				options = append(options, grpc.WithContextDialer(contextDialer))
			}
			var err error
			connection, err = grpc.NewClient(target, options...)
			if err != nil {
				lastError = err
				if !retryDial {
					return nil, nil, err
				}
				time.Sleep(commandClientDialDelay(attempt))
				continue
			}
			client = daemon.NewStartedServiceClient(connection)
		}
		waitDuration := commandClientDialDelay(attempt)
		ctx, cancel := context.WithTimeout(context.Background(), waitDuration)
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
	return nil, nil, lastError
}

func (c *CommandClient) Connect() error {
	c.clientMutex.Lock()
	common.Close(common.PtrOrNil(c.grpcConn))

	target, contextDialer := dialTarget()
	connection, client, err := c.dialWithRetry(target, contextDialer, true)
	if err != nil {
		c.clientMutex.Unlock()
		return err
	}
	c.grpcConn = connection
	c.grpcClient = client
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
	connection, client, err := c.dialWithRetry("passthrough:///xpc", func(ctx context.Context, _ string) (net.Conn, error) {
		return networkConnection, nil
	}, false)
	if err != nil {
		networkConnection.Close()
		c.clientMutex.Unlock()
		return err
	}
	c.grpcConn = connection
	c.grpcClient = client
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

func (c *CommandClient) getClientForCall() (daemon.StartedServiceClient, error) {
	c.clientMutex.RLock()
	if c.grpcClient != nil {
		defer c.clientMutex.RUnlock()
		return c.grpcClient, nil
	}
	c.clientMutex.RUnlock()

	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()

	if c.grpcClient != nil {
		return c.grpcClient, nil
	}

	target, contextDialer := dialTarget()
	connection, client, err := c.dialWithRetry(target, contextDialer, true)
	if err != nil {
		return nil, err
	}
	c.grpcConn = connection
	c.grpcClient = client
	if c.ctx == nil {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	}
	return c.grpcClient, nil
}

func (c *CommandClient) closeConnection() {
	c.clientMutex.Lock()
	defer c.clientMutex.Unlock()
	if c.grpcConn != nil {
		c.grpcConn.Close()
		c.grpcConn = nil
		c.grpcClient = nil
	}
}

func callWithResult[T any](c *CommandClient, call func(client daemon.StartedServiceClient) (T, error)) (T, error) {
	client, err := c.getClientForCall()
	if err != nil {
		var zero T
		return zero, err
	}
	if c.standalone {
		defer c.closeConnection()
	}
	return call(client)
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
		c.handler.Disconnected(err.Error())
		return
	}
	defaultLogLevel, err := client.GetDefaultLogLevel(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(err.Error())
		return
	}
	c.handler.SetDefaultLogLevel(int32(defaultLogLevel.Level))
	for {
		logMessage, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(err.Error())
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
		c.handler.Disconnected(err.Error())
		return
	}

	for {
		status, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		c.handler.WriteStatus(StatusMessageFromGRPC(status))
	}
}

func (c *CommandClient) handleGroupStream() {
	client, ctx := c.getStreamContext()

	stream, err := client.SubscribeGroups(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(err.Error())
		return
	}

	for {
		groups, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		c.handler.WriteGroups(OutboundGroupIteratorFromGRPC(groups))
	}
}

func (c *CommandClient) handleClashModeStream() {
	client, ctx := c.getStreamContext()

	modeStatus, err := client.GetClashModeStatus(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(err.Error())
		return
	}

	if sFixAndroidStack {
		go func() {
			c.handler.InitializeClashMode(newIterator(modeStatus.ModeList), modeStatus.CurrentMode)
			if len(modeStatus.ModeList) == 0 {
				c.handler.Disconnected(os.ErrInvalid.Error())
			}
		}()
	} else {
		c.handler.InitializeClashMode(newIterator(modeStatus.ModeList), modeStatus.CurrentMode)
		if len(modeStatus.ModeList) == 0 {
			c.handler.Disconnected(os.ErrInvalid.Error())
			return
		}
	}

	if len(modeStatus.ModeList) == 0 {
		return
	}

	stream, err := client.SubscribeClashMode(ctx, &emptypb.Empty{})
	if err != nil {
		c.handler.Disconnected(err.Error())
		return
	}

	for {
		mode, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(err.Error())
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
		c.handler.Disconnected(err.Error())
		return
	}

	for {
		events, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		libboxEvents := ConnectionEventsFromGRPC(events)
		c.handler.WriteConnectionEvents(libboxEvents)
	}
}

func (c *CommandClient) SelectOutbound(groupTag string, outboundTag string) error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SelectOutbound(context.Background(), &daemon.SelectOutboundRequest{
			GroupTag:    groupTag,
			OutboundTag: outboundTag,
		})
	})
	return err
}

func (c *CommandClient) URLTest(groupTag string) error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.URLTest(context.Background(), &daemon.URLTestRequest{
			OutboundTag: groupTag,
		})
	})
	return err
}

func (c *CommandClient) SetClashMode(newMode string) error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SetClashMode(context.Background(), &daemon.ClashMode{
			Mode: newMode,
		})
	})
	return err
}

func (c *CommandClient) CloseConnection(connId string) error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.CloseConnection(context.Background(), &daemon.CloseConnectionRequest{
			Id: connId,
		})
	})
	return err
}

func (c *CommandClient) CloseConnections() error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.CloseAllConnections(context.Background(), &emptypb.Empty{})
	})
	return err
}

func (c *CommandClient) ServiceReload() error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.ReloadService(context.Background(), &emptypb.Empty{})
	})
	return err
}

func (c *CommandClient) ServiceClose() error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.StopService(context.Background(), &emptypb.Empty{})
	})
	return err
}

func (c *CommandClient) ClearLogs() error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.ClearLogs(context.Background(), &emptypb.Empty{})
	})
	return err
}

func (c *CommandClient) GetSystemProxyStatus() (*SystemProxyStatus, error) {
	return callWithResult(c, func(client daemon.StartedServiceClient) (*SystemProxyStatus, error) {
		status, err := client.GetSystemProxyStatus(context.Background(), &emptypb.Empty{})
		if err != nil {
			return nil, err
		}
		return SystemProxyStatusFromGRPC(status), nil
	})
}

func (c *CommandClient) SetSystemProxyEnabled(isEnabled bool) error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SetSystemProxyEnabled(context.Background(), &daemon.SetSystemProxyEnabledRequest{
			Enabled: isEnabled,
		})
	})
	return err
}

func (c *CommandClient) GetDeprecatedNotes() (DeprecatedNoteIterator, error) {
	return callWithResult(c, func(client daemon.StartedServiceClient) (DeprecatedNoteIterator, error) {
		warnings, err := client.GetDeprecatedWarnings(context.Background(), &emptypb.Empty{})
		if err != nil {
			return nil, err
		}
		var notes []*DeprecatedNote
		for _, warning := range warnings.Warnings {
			notes = append(notes, &DeprecatedNote{
				Description:   warning.Message,
				MigrationLink: warning.MigrationLink,
			})
		}
		return newIterator(notes), nil
	})
}

func (c *CommandClient) GetStartedAt() (int64, error) {
	return callWithResult(c, func(client daemon.StartedServiceClient) (int64, error) {
		startedAt, err := client.GetStartedAt(context.Background(), &emptypb.Empty{})
		if err != nil {
			return 0, err
		}
		return startedAt.StartedAt, nil
	})
}

func (c *CommandClient) SetGroupExpand(groupTag string, isExpand bool) error {
	_, err := callWithResult(c, func(client daemon.StartedServiceClient) (*emptypb.Empty, error) {
		return client.SetGroupExpand(context.Background(), &daemon.SetGroupExpandRequest{
			GroupTag: groupTag,
			IsExpand: isExpand,
		})
	})
	return err
}
