//go:build linux || (darwin && cgo) || windows

package usbip

import (
	"context"
	"net"
	"sync"
	"time"

	E "github.com/sagernet/sing/common/exceptions"
)

var errClientControlSessionClosed = E.New("usbip control session closed")

type clientImportLease struct {
	Valid       bool
	ID          uint64
	ClientNonce uint64
}

type clientControlSession struct {
	conn         net.Conn
	capabilities uint32
	writeAccess  sync.Mutex
	access       sync.Mutex
	nextNonce    uint64
	pending      map[uint64]chan clientLeaseResult
	closed       bool
}

type clientLeaseResult struct {
	response controlLeaseResponse
	err      error
}

func newClientControlSession(conn net.Conn, capabilities uint32) *clientControlSession {
	return &clientControlSession{
		conn:         conn,
		capabilities: capabilities,
		pending:      make(map[uint64]chan clientLeaseResult),
	}
}

func (s *clientControlSession) writeControl(frame controlFrame, payload any) error {
	s.writeAccess.Lock()
	defer s.writeAccess.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(controlWriteTimeout))
	err := writeControlMessage(s.conn, frame, payload)
	_ = s.conn.SetWriteDeadline(time.Time{})
	return err
}

func (s *clientControlSession) requestLease(ctx context.Context, busid string) (controlLeaseResponse, error) {
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		return controlLeaseResponse{}, errClientControlSessionClosed
	}
	s.nextNonce++
	nonce := s.nextNonce
	waiter := make(chan clientLeaseResult, 1)
	s.pending[nonce] = waiter
	s.access.Unlock()

	request := controlLeaseRequest{
		BusID:       busid,
		ClientNonce: nonce,
	}
	err := s.writeControl(controlFrame{
		Type:    controlFrameLeaseRequest,
		Version: controlProtocolVersion,
	}, request)
	if err != nil {
		s.access.Lock()
		delete(s.pending, nonce)
		s.access.Unlock()
		return controlLeaseResponse{}, err
	}

	select {
	case result := <-waiter:
		return result.response, result.err
	case <-ctx.Done():
		s.access.Lock()
		delete(s.pending, nonce)
		s.access.Unlock()
		return controlLeaseResponse{}, ctx.Err()
	}
}

func (s *clientControlSession) deliverLeaseResponse(response controlLeaseResponse) bool {
	s.access.Lock()
	waiter, ok := s.pending[response.ClientNonce]
	if ok {
		delete(s.pending, response.ClientNonce)
	}
	s.access.Unlock()
	if !ok {
		return false
	}
	waiter <- clientLeaseResult{response: response}
	close(waiter)
	return true
}

func (s *clientControlSession) closeWithError(err error) {
	s.access.Lock()
	if s.closed {
		s.access.Unlock()
		return
	}
	s.closed = true
	pending := s.pending
	s.pending = make(map[uint64]chan clientLeaseResult)
	s.access.Unlock()

	for _, waiter := range pending {
		waiter <- clientLeaseResult{err: err}
		close(waiter)
	}
}

func (c *ClientService) requestImportLease(ctx context.Context, busid string) (clientImportLease, error) {
	c.controlAccess.Lock()
	session := c.controlSession
	c.controlAccess.Unlock()
	if session == nil || !supportsControlExtensions(session.capabilities) {
		return clientImportLease{}, nil
	}
	response, err := session.requestLease(ctx, busid)
	if err != nil {
		return clientImportLease{}, E.Cause(err, "request import lease")
	}
	if response.ErrorCode != "" {
		return clientImportLease{}, E.New("remote rejected import lease (", response.ErrorCode, ": ", response.ErrorMessage, ")")
	}
	return clientImportLease{
		Valid:       true,
		ID:          response.LeaseID,
		ClientNonce: response.ClientNonce,
	}, nil
}

func (c *ClientService) applyControlDelta(delta controlDeviceDelta) {
	c.remoteAccess.Lock()
	if c.remoteDevicesV2 == nil {
		c.remoteDevicesV2 = make(map[string]DeviceInfoV2)
	}
	for _, busid := range delta.Removed {
		delete(c.remoteDevicesV2, busid)
	}
	for _, device := range delta.Added {
		if device.BusID == "" {
			continue
		}
		c.remoteDevicesV2[device.BusID] = device
	}
	for _, device := range delta.Updated {
		if device.BusID == "" {
			continue
		}
		c.remoteDevicesV2[device.BusID] = device
	}
	values := sortedDeviceInfoV2Values(c.remoteDevicesV2)
	c.remoteAccess.Unlock()
	c.applyRemoteDeviceState(values)
}

func (c *ClientService) syncRemoteStateAndResetControlState(ctx context.Context) error {
	entries, err := c.fetchDevList(ctx)
	if err != nil {
		return err
	}
	devices := make(map[string]DeviceInfoV2, len(entries))
	for _, entry := range entries {
		device := deviceInfoV2FromEntry(entry, "", "", deviceStateAvailable, 0, deviceStateAvailable)
		if device.BusID == "" {
			continue
		}
		devices[device.BusID] = device
	}
	c.remoteAccess.Lock()
	c.remoteDevicesV2 = devices
	c.remoteAccess.Unlock()
	if !c.assignment.Matched() {
		c.applyRemoteExports(entries)
		return nil
	}
	c.applyMatchedExportsWithRetained(entries, nil)
	return nil
}
