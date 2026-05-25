//go:build darwin && cgo

package usbip

// Close is a no-op: darwinExportHost owns the device handle across attachments.
type darwinIOUSBHostEngine struct {
	device *darwinUSBHostDevice
}

func newDarwinIOUSBHostEngine(device *darwinUSBHostDevice) *darwinIOUSBHostEngine {
	return &darwinIOUSBHostEngine{device: device}
}

func (e *darwinIOUSBHostEngine) Submit(req URBRequest) URBResponse {
	command := req.Command
	switch {
	case command.Header.Endpoint == 0:
		status, actual, outBuf, err := e.device.control(command.Setup, req.Buffer)
		return URBResponse{Status: status, ActualLength: actual, Buffer: outBuf, Error: err}
	case command.NumberOfPackets > 0:
		asap := command.TransferFlags&usbipTransferFlagIsoASAP != 0
		status, actual, outBuf, isoOut, err := e.device.iso(req.Endpoint, req.Buffer, command.StartFrame, asap, req.IsoPackets)
		return URBResponse{Status: status, ActualLength: actual, Buffer: outBuf, IsoPackets: isoOut, Error: err}
	default:
		status, actual, outBuf, err := e.device.io(req.Endpoint, req.Buffer)
		return URBResponse{Status: status, ActualLength: actual, Buffer: outBuf, Error: err}
	}
}

func (e *darwinIOUSBHostEngine) AbortEndpoint(endpoint uint8) error {
	return e.device.abortEndpoint(endpoint)
}

func (e *darwinIOUSBHostEngine) Close() error {
	return nil
}
