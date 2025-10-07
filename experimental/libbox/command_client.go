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
	WriteConnections(message *Connections)
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

func NewStandaloneCommandClient() *CommandClient {
	return new(CommandClient)
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

func (c *CommandClient) grpcDial() (*grpc.ClientConn, error) {
	var target string
	if sCommandServerListenPort == 0 {
		target = "unix://" + filepath.Join(sBasePath, "command.sock")
	} else {
		target = net.JoinHostPort("127.0.0.1", strconv.Itoa(int(sCommandServerListenPort)))
	}
	var (
		conn *grpc.ClientConn
		err  error
	)
	clientOptions := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(unaryClientAuthInterceptor),
		grpc.WithStreamInterceptor(streamClientAuthInterceptor),
	}
	for i := 0; i < 10; i++ {
		conn, err = grpc.NewClient(target, clientOptions...)
		if err == nil {
			return conn, nil
		}
		time.Sleep(time.Duration(100+i*50) * time.Millisecond)
	}
	return nil, err
}

func (c *CommandClient) Connect() error {
	c.clientMutex.Lock()
	common.Close(common.PtrOrNil(c.grpcConn))

	conn, err := c.grpcDial()
	if err != nil {
		c.clientMutex.Unlock()
		return err
	}
	c.grpcConn = conn
	c.grpcClient = daemon.NewStartedServiceClient(conn)
	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.clientMutex.Unlock()

	c.handler.Connected()
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

	conn, err := c.grpcDial()
	if err != nil {
		return nil, err
	}
	c.grpcConn = conn
	c.grpcClient = daemon.NewStartedServiceClient(conn)
	if c.ctx == nil {
		c.ctx, c.cancel = context.WithCancel(context.Background())
	}
	return c.grpcClient, nil
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

	var connections Connections
	for {
		conns, err := stream.Recv()
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		connections.input = ConnectionsFromGRPC(conns)
		c.handler.WriteConnections(&connections)
	}
}

func (c *CommandClient) SelectOutbound(groupTag string, outboundTag string) error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.SelectOutbound(context.Background(), &daemon.SelectOutboundRequest{
		GroupTag:    groupTag,
		OutboundTag: outboundTag,
	})
	return err
}

func (c *CommandClient) URLTest(groupTag string) error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.URLTest(context.Background(), &daemon.URLTestRequest{
		OutboundTag: groupTag,
	})
	return err
}

func (c *CommandClient) SetClashMode(newMode string) error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.SetClashMode(context.Background(), &daemon.ClashMode{
		Mode: newMode,
	})
	return err
}

func (c *CommandClient) CloseConnection(connId string) error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.CloseConnection(context.Background(), &daemon.CloseConnectionRequest{
		Id: connId,
	})
	return err
}

func (c *CommandClient) CloseConnections() error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.CloseAllConnections(context.Background(), &emptypb.Empty{})
	return err
}

func (c *CommandClient) ServiceReload() error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.ReloadService(context.Background(), &emptypb.Empty{})
	return err
}

func (c *CommandClient) ServiceClose() error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.StopService(context.Background(), &emptypb.Empty{})
	return err
}

func (c *CommandClient) GetSystemProxyStatus() (*SystemProxyStatus, error) {
	client, err := c.getClientForCall()
	if err != nil {
		return nil, err
	}

	status, err := client.GetSystemProxyStatus(context.Background(), &emptypb.Empty{})
	if err != nil {
		return nil, err
	}
	return SystemProxyStatusFromGRPC(status), nil
}

func (c *CommandClient) SetSystemProxyEnabled(isEnabled bool) error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.SetSystemProxyEnabled(context.Background(), &daemon.SetSystemProxyEnabledRequest{
		Enabled: isEnabled,
	})
	return err
}

func (c *CommandClient) GetDeprecatedNotes() (DeprecatedNoteIterator, error) {
	client, err := c.getClientForCall()
	if err != nil {
		return nil, err
	}

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
}

func (c *CommandClient) SetGroupExpand(groupTag string, isExpand bool) error {
	client, err := c.getClientForCall()
	if err != nil {
		return err
	}

	_, err = client.SetGroupExpand(context.Background(), &daemon.SetGroupExpandRequest{
		GroupTag: groupTag,
		IsExpand: isExpand,
	})
	return err
}
