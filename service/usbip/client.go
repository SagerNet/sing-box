//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
	"fmt"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	boxService "github.com/sagernet/sing-box/adapter/service"
	"github.com/sagernet/sing-box/common/dialer"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

type ClientService struct {
	boxService.Adapter
	ctx        context.Context
	cancel     context.CancelFunc
	logger     log.ContextLogger
	dialer     N.Dialer
	serverAddr M.Socksaddr
	host       ImportHost

	assignment      *clientAssignment
	workerAccess    sync.Mutex
	assignedWorkers []*clientAssignedWorker
	allWorkers      map[string]context.CancelFunc

	controlAccess  sync.Mutex
	controlSession *clientControlSession

	remoteAccess    sync.Mutex
	remoteDevicesV2 map[string]DeviceInfoV2
}

func NewClientService(ctx context.Context, logger log.ContextLogger, tag string, options option.USBIPClientServiceOptions) (adapter.Service, error) {
	for i, m := range options.Devices {
		if m.IsZero() {
			return nil, E.New("devices[", i, "]: at least one of busid/vendor_id/product_id/serial is required")
		}
	}
	if options.ServerPort == 0 {
		options.ServerPort = DefaultPort
	}
	if options.Server == "" {
		return nil, E.New("missing server address")
	}
	outboundDialer, err := dialer.New(ctx, options.DialerOptions, options.ServerOptions.ServerIsDomain())
	if err != nil {
		return nil, err
	}
	host, err := newPlatformImportHost(logger)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	return &ClientService{
		Adapter:    boxService.NewAdapter(C.TypeUSBIPClient, tag),
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		dialer:     outboundDialer,
		serverAddr: options.ServerOptions.Build(),
		host:       host,
		assignment: newClientAssignment(options.Devices),
		allWorkers: make(map[string]context.CancelFunc),
	}, nil
}

func (c *ClientService) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateStart {
		return nil
	}
	err := c.host.Start()
	if err != nil {
		return err
	}
	c.initializeWorkers()
	go c.run()
	return nil
}

func (c *ClientService) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	_ = c.host.Close()
	return nil
}

func (c *ClientService) runBusIDLoop(ctx context.Context, busid, description string) {
	for {
		if ctx.Err() != nil {
			return
		}
		c.assignment.SetActive(busid, true)
		session, err := c.attemptAttach(ctx, busid)
		if err != nil {
			c.assignment.SetActive(busid, false)
			c.logger.Error("attach ", description, " (", busid, "): ", err)
			if !sleepCtx(ctx, clientReconnectDelay) {
				return
			}
			continue
		}
		c.logger.Info("attached ", busid, " through ", session.Description())
		select {
		case <-session.Done():
			c.logger.Debug("import session for ", busid, " ended (", session.Description(), ")")
		case <-ctx.Done():
			_ = session.Close()
			<-session.Done()
		}
		_ = session.Close()
		c.assignment.SetActive(busid, false)
		if ctx.Err() != nil {
			return
		}
		retry := true
		if !c.assignment.Matched() {
			err = c.syncRemoteStateContext(ctx)
			if err != nil {
				c.logger.Warn("refresh remote exports after releasing ", busid, ": ", err)
			} else {
				retry = c.assignment.IsRetryDesired(busid)
			}
		}
		if !retry {
			c.logger.Info("remote export ", busid, " disappeared; stopping import worker")
			return
		}
		if !sleepCtx(ctx, clientReconnectDelay) {
			return
		}
	}
}

func (c *ClientService) attemptAttach(ctx context.Context, busid string) (AttachedSession, error) {
	conn, err := c.dialer.DialContext(ctx, N.NetworkTCP, c.serverAddr)
	if err != nil {
		return nil, E.Cause(err, "dial ", c.serverAddr)
	}
	releaseConn := true
	defer func() {
		if releaseConn {
			_ = conn.Close()
		}
	}()
	stopCloseOnCancel := closeConnOnContextDone(ctx, conn)
	defer stopCloseOnCancel()

	lease, err := c.requestImportLease(ctx, busid)
	if err != nil {
		return nil, err
	}
	expectedReply := OpRepImport
	if lease.Valid {
		expectedReply = OpRepImportExt
		err = WriteOpReqImportExt(conn, ImportExtRequest{
			BusID:       busid,
			LeaseID:     lease.ID,
			ClientNonce: lease.ClientNonce,
		})
		if err != nil {
			return nil, E.Cause(err, "write OP_REQ_IMPORT_EXT")
		}
	} else {
		err = WriteOpReqImport(conn, busid)
		if err != nil {
			return nil, E.Cause(err, "write OP_REQ_IMPORT")
		}
	}
	header, err := ReadOpHeader(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_IMPORT header")
	}
	if header.Version != ProtocolVersion {
		return nil, E.New("unexpected reply version ", fmt.Sprintf("0x%04x", header.Version))
	}
	if header.Code != expectedReply {
		return nil, E.New("unexpected reply code ", fmt.Sprintf("0x%04x", header.Code))
	}
	if header.Status != OpStatusOK {
		return nil, E.New("remote rejected import (status=", header.Status, ")")
	}
	info, err := ReadOpRepImportBody(conn)
	if err != nil {
		return nil, E.Cause(err, "read OP_REP_IMPORT body")
	}
	session, err := c.host.Attach(ctx, info, conn)
	if err != nil {
		return nil, err
	}
	releaseConn = false
	return session, nil
}
