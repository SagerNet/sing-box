//go:build darwin && cgo

package usbip

import (
	"context"
	"errors"
	"unsafe"

	"github.com/sagernet/sing-box/log"

	"golang.org/x/sys/unix"
)

type darwinEndpoint struct {
	ctx    context.Context
	cancel context.CancelFunc
	logger log.ContextLogger

	sm    *darwinUSBHostEndpointSM
	peer  *UsbIpPeer
	iso   *IsoScheduler
	devID uint32
	key   darwinEndpointKey

	cmdCh      chan darwinCIMessage
	doorbellCh chan uint32
	workerDone chan struct{}

	setup    [8]byte
	setupSet bool
}

type pendingTransfer struct {
	transaction *UrbTransaction
	transfer    darwinCITransfer
	direction   uint32
	requestLen  int
	bufferPtr   unsafe.Pointer
	noResponse  bool
}

func newDarwinEndpoint(ctx context.Context, logger log.ContextLogger, sm *darwinUSBHostEndpointSM, peer *UsbIpPeer, iso *IsoScheduler, devID uint32, key darwinEndpointKey) *darwinEndpoint {
	ctx, cancel := context.WithCancel(ctx)
	e := &darwinEndpoint{
		ctx:        ctx,
		cancel:     cancel,
		logger:     logger,
		sm:         sm,
		peer:       peer,
		iso:        iso,
		devID:      devID,
		key:        key,
		cmdCh:      make(chan darwinCIMessage, 4),
		doorbellCh: make(chan uint32, 16),
		workerDone: make(chan struct{}),
	}
	go e.worker()
	return e
}

func (e *darwinEndpoint) Command(message darwinCIMessage) {
	select {
	case e.cmdCh <- message:
	case <-e.ctx.Done():
	}
}

func (e *darwinEndpoint) EnqueueDoorbell(doorbell uint32) {
	select {
	case e.doorbellCh <- doorbell:
	case <-e.ctx.Done():
	default:
		e.logger.Debug("endpoint doorbell queue overflow ", e.key.device, ".", e.key.endpoint)
	}
}

func (e *darwinEndpoint) Close() {
	e.cancel()
	<-e.workerDone
}

func (e *darwinEndpoint) Wait() {
	<-e.workerDone
}

func (e *darwinEndpoint) worker() {
	defer close(e.workerDone)
	defer e.sm.Close()

	var pending *pendingTransfer
	var pendingDoorbell bool
	var previousNoResponse unsafe.Pointer

	for {
		if pending != nil {
			select {
			case <-e.ctx.Done():
				e.abortPending(pending)
				return
			case message := <-e.cmdCh:
				e.abortPending(pending)
				pending = nil
				if e.handleCommand(message) {
					return
				}
				pendingDoorbell = false
				previousNoResponse = nil
			case <-pending.transaction.Done():
				e.finalizePending(pending)
				pending = nil
				pendingDoorbell = true
			}
			continue
		}

		if pendingDoorbell {
			transfer := e.sm.currentTransfer()
			if transfer.ptr == nil || !transfer.message.valid() {
				pendingDoorbell = false
				continue
			}
			noResponse := transfer.message.control&(1<<14) != 0
			if noResponse {
				if transfer.ptr == previousNoResponse {
					pendingDoorbell = false
					continue
				}
				previousNoResponse = transfer.ptr
			} else {
				previousNoResponse = nil
			}
			pending = e.startTransfer(transfer, noResponse)
			continue
		}

		select {
		case <-e.ctx.Done():
			return
		case message := <-e.cmdCh:
			if e.handleCommand(message) {
				return
			}
		case doorbell := <-e.doorbellCh:
			err := e.sm.processDoorbell(doorbell)
			if err != nil {
				e.logger.Debug("process doorbell: ", err)
				continue
			}
			pendingDoorbell = true
		}
	}
}

func (e *darwinEndpoint) handleCommand(message darwinCIMessage) bool {
	err := e.sm.respond(message, ciStatusSuccess)
	if err != nil {
		e.logger.Debug("respond endpoint command: ", err)
	}
	return message.messageType() == ciMsgEndpointDestroy
}

func (e *darwinEndpoint) abortPending(pending *pendingTransfer) {
	_ = pending.transaction.Cancel(context.Background())
	select {
	case <-pending.transaction.Done():
	case <-e.peer.Done():
	}
	e.finalizePending(pending)
}

func (e *darwinEndpoint) finalizePending(pending *pendingTransfer) {
	response, err := pending.transaction.Wait(context.Background())
	var status int32
	var length int
	switch {
	case errors.Is(err, ErrCanceled):
		status = -int32(unix.ECONNRESET)
		length = 0
	case errors.Is(err, ErrPeerClosed):
		status = -int32(unix.ENODEV)
		length = 0
	case err != nil:
		status = -int32(unix.EIO)
		length = 0
	default:
		status, length = e.scatterResponse(pending, response)
	}
	if pending.noResponse {
		return
	}
	completeErr := e.sm.complete(pending.transfer, darwinUSBIPStatusToCIStatus(status), length)
	if completeErr != nil {
		e.logger.Debug("complete transfer: ", completeErr)
		e.cancel()
	}
}

func (e *darwinEndpoint) scatterResponse(pending *pendingTransfer, response SubmitResponse) (int32, int) {
	if pending.direction != USBIPDirIn {
		return response.Status, int(response.ActualLength)
	}
	if response.ActualLength < 0 {
		e.logger.Debug("RET_SUBMIT actual_length is negative: ", response.ActualLength)
		e.cancel()
		return -int32(unix.EPROTO), 0
	}
	actualLength := int(response.ActualLength)
	if actualLength > pending.requestLen || len(response.Buffer) > pending.requestLen {
		e.logger.Debug("RET_SUBMIT actual_length ", actualLength, " exceeds request length ", pending.requestLen)
		e.cancel()
		return -int32(unix.EOVERFLOW), 0
	}
	copyLength := min(actualLength, len(response.Buffer))
	if copyLength > 0 && pending.bufferPtr != nil {
		if len(response.IsoPackets) > 0 {
			dst := unsafe.Slice((*byte)(pending.bufferPtr), pending.requestLen)
			ScatterIsoResponse(dst, response.Buffer[:copyLength], response.IsoPackets)
		} else {
			copy(unsafe.Slice((*byte)(pending.bufferPtr), copyLength), response.Buffer[:copyLength])
		}
	}
	return response.Status, actualLength
}

func (e *darwinEndpoint) startTransfer(transfer darwinCITransfer, noResponse bool) *pendingTransfer {
	message := transfer.message
	switch message.messageType() {
	case ciMsgSetupTransfer:
		e.setup = message.setup()
		e.setupSet = true
		if !noResponse {
			err := e.sm.complete(transfer, ciStatusSuccess, 0)
			if err != nil {
				e.logger.Debug("complete setup transfer: ", err)
				e.cancel()
			}
		}
		return nil
	case ciMsgStatusTransfer:
		return e.startControlStatusTransfer(transfer, noResponse)
	case ciMsgNormalTransfer:
		if e.key.endpoint == 0 {
			return e.startControlDataTransfer(transfer, message, noResponse)
		}
		return e.startNormalTransfer(transfer, message, noResponse)
	case ciMsgIsochronousTransfer:
		return e.startIsoTransfer(transfer, message, noResponse)
	default:
		if !noResponse {
			_ = e.sm.complete(transfer, darwinUSBIPStatusToCIStatus(-int32(unix.EIO)), 0)
		}
		return nil
	}
}

func (e *darwinEndpoint) startControlStatusTransfer(transfer darwinCITransfer, noResponse bool) *pendingTransfer {
	if !e.setupSet {
		if !noResponse {
			_ = e.sm.complete(transfer, ciStatusSuccess, 0)
		}
		return nil
	}
	setup := e.setup
	e.setupSet = false
	if setup[6] != 0 || setup[7] != 0 {
		if !noResponse {
			_ = e.sm.complete(transfer, ciStatusSuccess, 0)
		}
		return nil
	}
	transaction, err := e.peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     e.devID,
			Direction: USBIPDirOut,
			Endpoint:  0,
		},
		NumberOfPackets: nonIsoPacketCount,
		Setup:           setup,
	})
	if err != nil {
		if !noResponse {
			_ = e.sm.complete(transfer, darwinUSBIPStatusToCIStatus(-int32(unix.EIO)), 0)
		}
		return nil
	}
	return &pendingTransfer{
		transaction: transaction,
		transfer:    transfer,
		direction:   USBIPDirOut,
		noResponse:  noResponse,
	}
}

func (e *darwinEndpoint) startControlDataTransfer(transfer darwinCITransfer, message darwinCIMessage, noResponse bool) *pendingTransfer {
	if !e.setupSet {
		if !noResponse {
			_ = e.sm.complete(transfer, darwinUSBIPStatusToCIStatus(-int32(unix.EPROTO)), 0)
		}
		return nil
	}
	setup := e.setup
	length := int(message.normalLength())
	direction := USBIPDirOut
	var buffer []byte
	if setup[0]&0x80 != 0 {
		direction = USBIPDirIn
	} else {
		buffer = bytesFromUnsafe(message.bufferPointer(), length)
	}
	transaction, err := e.peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     e.devID,
			Direction: direction,
			Endpoint:  0,
		},
		TransferBufferLength: int32(length),
		NumberOfPackets:      nonIsoPacketCount,
		Setup:                setup,
		Buffer:               buffer,
	})
	if err != nil {
		if !noResponse {
			_ = e.sm.complete(transfer, darwinUSBIPStatusToCIStatus(-int32(unix.EIO)), 0)
		}
		return nil
	}
	return &pendingTransfer{
		transaction: transaction,
		transfer:    transfer,
		direction:   direction,
		requestLen:  length,
		bufferPtr:   message.bufferPointer(),
		noResponse:  noResponse,
	}
}

func (e *darwinEndpoint) startNormalTransfer(transfer darwinCITransfer, message darwinCIMessage, noResponse bool) *pendingTransfer {
	length := int(message.normalLength())
	direction := USBIPDirOut
	var buffer []byte
	if e.key.endpoint&0x80 != 0 {
		direction = USBIPDirIn
	} else {
		buffer = bytesFromUnsafe(message.bufferPointer(), length)
	}
	transaction, err := e.peer.Submit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     e.devID,
			Direction: direction,
			Endpoint:  uint32(e.key.endpoint & 0x0f),
		},
		TransferBufferLength: int32(length),
		NumberOfPackets:      nonIsoPacketCount,
		Buffer:               buffer,
	})
	if err != nil {
		if !noResponse {
			_ = e.sm.complete(transfer, darwinUSBIPStatusToCIStatus(-int32(unix.EIO)), 0)
		}
		return nil
	}
	return &pendingTransfer{
		transaction: transaction,
		transfer:    transfer,
		direction:   direction,
		requestLen:  length,
		bufferPtr:   message.bufferPointer(),
		noResponse:  noResponse,
	}
}

func (e *darwinEndpoint) startIsoTransfer(transfer darwinCITransfer, message darwinCIMessage, noResponse bool) *pendingTransfer {
	length := int(message.normalLength())
	direction := USBIPDirOut
	var buffer []byte
	if e.key.endpoint&0x80 != 0 {
		direction = USBIPDirIn
	} else {
		buffer = bytesFromUnsafe(message.bufferPointer(), length)
	}
	ciFrame := uint8(message.control >> ciIsochronousTransferControlFramePhase)
	asap := message.control&ciIsochronousTransferControlASAP != 0
	command := e.iso.EncodeSubmit(SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			DevID:     e.devID,
			Direction: direction,
			Endpoint:  uint32(e.key.endpoint & 0x0f),
		},
		TransferBufferLength: int32(length),
		NumberOfPackets:      1,
		Buffer:               buffer,
		IsoPackets: []IsoPacketDescriptor{{
			Offset: 0,
			Length: int32(length),
		}},
	}, ciFrame, asap)
	transaction, err := e.peer.Submit(command)
	if err != nil {
		if !noResponse {
			_ = e.sm.complete(transfer, darwinUSBIPStatusToCIStatus(-int32(unix.EIO)), 0)
		}
		return nil
	}
	return &pendingTransfer{
		transaction: transaction,
		transfer:    transfer,
		direction:   direction,
		requestLen:  length,
		bufferPtr:   message.bufferPointer(),
		noResponse:  noResponse,
	}
}

func bytesFromUnsafe(ptr unsafe.Pointer, length int) []byte {
	if ptr == nil || length == 0 {
		return nil
	}
	out := make([]byte, length)
	copy(out, unsafe.Slice((*byte)(ptr), length))
	return out
}
