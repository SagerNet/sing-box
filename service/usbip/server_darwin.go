//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"slices"
	"sync"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"

	"golang.org/x/sys/unix"
)

func newPlatformExportHost(logger log.ContextLogger, matches []option.USBIPDeviceMatch) (ExportHost, error) {
	return newDarwinExportHost(logger, matches, systemDarwinServerOps), nil
}

func newPlatformImportHost(logger log.ContextLogger) (ImportHost, error) {
	return newDarwinImportHost(logger), nil
}

type darwinUSBHostDeviceWatch interface {
	Close()
}

type darwinServerOps struct {
	copyUSBHostDevices  func() ([]darwinUSBHostDeviceInfo, error)
	openUSBHostDevice   func(registryID uint64, capture bool) (*darwinUSBHostDevice, error)
	watchUSBHostDevices func(func()) (darwinUSBHostDeviceWatch, error)
}

var systemDarwinServerOps = darwinServerOps{
	copyUSBHostDevices:  darwinCopyUSBHostDevices,
	openUSBHostDevice:   darwinOpenUSBHostDevice,
	watchUSBHostDevices: darwinWatchUSBHostDevices,
}

func darwinStableID(registryID uint64) string {
	return fmt.Sprintf("darwin-registry:%016x", registryID)
}

var _ DataSession = (*darwinServerDataSession)(nil)

type darwinServerDataSession struct {
	ctx         context.Context
	logger      log.ContextLogger
	conn        net.Conn
	device      darwinServerDataDevice
	writeAccess sync.Mutex
	access      sync.Mutex
	pending     map[uint32]darwinServerPendingSubmit
	wg          sync.WaitGroup

	done      chan struct{}
	doneOnce  sync.Once
	runErr    error
	closeOnce sync.Once
	closeErr  error
}

type darwinServerPendingSubmit struct {
	endpoint uint8
	unlinked bool
}

type darwinServerDataDevice interface {
	control(setup [8]byte, buffer []byte) (int32, int32, []byte, error)
	io(endpoint uint8, buffer []byte) (int32, int32, []byte, error)
	iso(endpoint uint8, buffer []byte, startFrame int32, packets []IsoPacketDescriptor) (int32, int32, []byte, []IsoPacketDescriptor, error)
	abortEndpoint(endpoint uint8) error
}

func newDarwinServerDataSession(ctx context.Context, logger log.ContextLogger, conn net.Conn, device darwinServerDataDevice) *darwinServerDataSession {
	session := &darwinServerDataSession{
		ctx:     ctx,
		logger:  logger,
		conn:    conn,
		device:  device,
		pending: make(map[uint32]darwinServerPendingSubmit),
		done:    make(chan struct{}),
	}
	go session.run()
	return session
}

func (s *darwinServerDataSession) Done() <-chan struct{} {
	return s.done
}

func (s *darwinServerDataSession) Err() error {
	return s.runErr
}

func (s *darwinServerDataSession) Close() error {
	s.closeOnce.Do(func() {
		s.closeErr = common.Close(s.conn)
	})
	<-s.done
	return s.closeErr
}

func (s *darwinServerDataSession) markDone(err error) {
	s.doneOnce.Do(func() {
		s.runErr = err
		close(s.done)
	})
}

func (s *darwinServerDataSession) run() {
	err := s.serve()
	if err != nil && (errors.Is(err, io.EOF) || E.IsClosedOrCanceled(err)) {
		err = nil
	}
	s.markDone(err)
}

func (s *darwinServerDataSession) serve() error {
	stopCloseOnCancel := closeConnOnContextDone(s.ctx, s.conn)
	defer stopCloseOnCancel()
	defer func() {
		s.abortPendingSubmits()
		s.wg.Wait()
	}()
	for {
		header, err := ReadDataHeader(s.conn)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		switch header.Command {
		case CmdSubmit:
			command, err := ReadSubmitCommandBody(s.conn, header)
			if err != nil {
				return err
			}
			s.trackSubmit(command.Header.SeqNum, commandEndpoint(command))
			s.wg.Add(1)
			go func() {
				defer s.wg.Done()
				response := s.handleSubmit(command)
				if !s.finishSubmit(command.Header.SeqNum) {
					return
				}
				s.writeAccess.Lock()
				err := WriteSubmitResponse(s.conn, response)
				s.writeAccess.Unlock()
				if err != nil {
					_ = s.conn.Close()
				}
			}()
		case CmdUnlink:
			command, err := ReadUnlinkCommandBody(s.conn, header)
			if err != nil {
				return err
			}
			status := int32(0)
			if endpoint, ok := s.markSubmitUnlinked(command.SeqNum); ok {
				abortErr := s.device.abortEndpoint(endpoint)
				if abortErr != nil {
					s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", abortErr)
				}
				status = usbipStatusECONNRESET
			}
			s.writeAccess.Lock()
			err = WriteUnlinkResponse(s.conn, UnlinkResponse{
				Header: DataHeader{Command: RetUnlink, SeqNum: header.SeqNum, DevID: header.DevID, Direction: header.Direction, Endpoint: header.Endpoint},
				Status: status,
			})
			s.writeAccess.Unlock()
			if err != nil {
				return err
			}
		default:
			return E.New(fmt.Sprintf("unexpected USB/IP command 0x%08x", header.Command))
		}
	}
}

func (s *darwinServerDataSession) handleSubmit(command SubmitCommand) SubmitResponse {
	response := SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    command.Header.SeqNum,
			DevID:     command.Header.DevID,
			Direction: command.Header.Direction,
			Endpoint:  command.Header.Endpoint,
		},
		StartFrame:      command.StartFrame,
		NumberOfPackets: command.NumberOfPackets,
		IsoPackets:      cloneIsoPackets(command.IsoPackets),
	}
	buffer := command.Buffer
	if command.Header.Direction == USBIPDirIn && command.TransferBufferLength > 0 {
		buffer = make([]byte, int(command.TransferBufferLength))
	}
	var (
		status int32
		actual int32
		err    error
	)
	endpoint := commandEndpoint(command)
	if command.Header.Endpoint == 0 {
		status, actual, buffer, err = s.device.control(command.Setup, buffer)
	} else if command.NumberOfPackets > 0 {
		status, actual, buffer, response.IsoPackets, err = s.device.iso(endpoint, buffer, command.StartFrame, response.IsoPackets)
	} else {
		status, actual, buffer, err = s.device.io(endpoint, buffer)
	}
	if err != nil {
		s.logger.Debug("submit seq ", command.Header.SeqNum, " endpoint 0x", hex8(endpoint), ": ", err)
		response.Status = -int32(unix.EIO)
		return response
	}
	response.Status = status
	if actual < 0 {
		actual = 0
	}
	response.ActualLength = actual
	if command.Header.Direction == USBIPDirIn && actual > 0 {
		response.Buffer = buffer[:min(int(actual), len(buffer))]
	}
	return response
}

func (s *darwinServerDataSession) trackSubmit(seq uint32, endpoint uint8) {
	s.access.Lock()
	defer s.access.Unlock()
	s.pending[seq] = darwinServerPendingSubmit{endpoint: endpoint}
}

func (s *darwinServerDataSession) markSubmitUnlinked(seq uint32) (uint8, bool) {
	s.access.Lock()
	defer s.access.Unlock()
	pending, ok := s.pending[seq]
	if !ok {
		return 0, false
	}
	pending.unlinked = true
	s.pending[seq] = pending
	return pending.endpoint, true
}

func (s *darwinServerDataSession) finishSubmit(seq uint32) bool {
	s.access.Lock()
	defer s.access.Unlock()
	pending, ok := s.pending[seq]
	if !ok {
		return true
	}
	delete(s.pending, seq)
	return !pending.unlinked
}

func (s *darwinServerDataSession) abortPendingSubmits() {
	endpoints := s.markPendingSubmitsUnlinked()
	if s.device == nil {
		return
	}
	for _, endpoint := range endpoints {
		err := s.device.abortEndpoint(endpoint)
		if err != nil {
			s.logger.Debug("abort endpoint 0x", hex8(endpoint), ": ", err)
		}
	}
}

func (s *darwinServerDataSession) markPendingSubmitsUnlinked() []uint8 {
	s.access.Lock()
	defer s.access.Unlock()
	seen := make(map[uint8]struct{})
	for seq, pending := range s.pending {
		if !pending.unlinked {
			seen[pending.endpoint] = struct{}{}
		}
		pending.unlinked = true
		s.pending[seq] = pending
	}
	endpoints := make([]uint8, 0, len(seen))
	for endpoint := range seen {
		endpoints = append(endpoints, endpoint)
	}
	slices.Sort(endpoints)
	return endpoints
}

func commandEndpoint(command SubmitCommand) uint8 {
	endpoint := uint8(command.Header.Endpoint & 0x0f)
	if command.Header.Direction == USBIPDirIn {
		endpoint |= 0x80
	}
	return endpoint
}

func cloneIsoPackets(in []IsoPacketDescriptor) []IsoPacketDescriptor {
	if len(in) == 0 {
		return nil
	}
	out := make([]IsoPacketDescriptor, len(in))
	copy(out, in)
	return out
}
