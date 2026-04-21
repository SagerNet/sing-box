package usbip

import (
	"encoding/binary"
	"io"
)

const (
	controlProtocolVersion uint8 = 1

	controlFrameHello   uint8 = 1
	controlFrameAck     uint8 = 2
	controlFrameChanged uint8 = 3
	controlFramePing    uint8 = 4
	controlFramePong    uint8 = 5

	controlCapabilityChanged  uint32 = 1 << 0
	controlCapabilityPingPong uint32 = 1 << 1
	controlCapabilities              = controlCapabilityChanged | controlCapabilityPingPong

	controlPrefaceSize = 8
	controlFrameSize   = 16
)

var controlPreface = [controlPrefaceSize]byte{'S', 'B', 'U', 'S', 'B', 'I', 'P', '1'}

type controlFrame struct {
	Type         uint8
	Version      uint8
	_            uint16
	Capabilities uint32
	Sequence     uint64
}

func WriteControlPreface(w io.Writer) error {
	_, err := w.Write(controlPreface[:])
	return err
}

func IsControlPreface(raw []byte) bool {
	if len(raw) != len(controlPreface) {
		return false
	}
	for i := range controlPreface {
		if raw[i] != controlPreface[i] {
			return false
		}
	}
	return true
}

func WriteControlHello(w io.Writer) error {
	return writeControlFrame(w, controlFrame{
		Type:         controlFrameHello,
		Version:      controlProtocolVersion,
		Capabilities: controlCapabilities,
	})
}

func WriteControlAck(w io.Writer, sequence uint64) error {
	return writeControlFrame(w, controlFrame{
		Type:         controlFrameAck,
		Version:      controlProtocolVersion,
		Capabilities: controlCapabilities,
		Sequence:     sequence,
	})
}

func WriteControlChanged(w io.Writer, sequence uint64) error {
	return writeControlFrame(w, controlFrame{
		Type:     controlFrameChanged,
		Version:  controlProtocolVersion,
		Sequence: sequence,
	})
}

func WriteControlPing(w io.Writer) error {
	return writeControlFrame(w, controlFrame{
		Type:    controlFramePing,
		Version: controlProtocolVersion,
	})
}

func WriteControlPong(w io.Writer) error {
	return writeControlFrame(w, controlFrame{
		Type:    controlFramePong,
		Version: controlProtocolVersion,
	})
}

func ReadControlFrame(r io.Reader) (controlFrame, error) {
	var raw [controlFrameSize]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return controlFrame{}, err
	}
	return controlFrame{
		Type:         raw[0],
		Version:      raw[1],
		Capabilities: binary.BigEndian.Uint32(raw[4:8]),
		Sequence:     binary.BigEndian.Uint64(raw[8:16]),
	}, nil
}

func writeControlFrame(w io.Writer, frame controlFrame) error {
	var raw [controlFrameSize]byte
	raw[0] = frame.Type
	raw[1] = frame.Version
	binary.BigEndian.PutUint32(raw[4:8], frame.Capabilities)
	binary.BigEndian.PutUint64(raw[8:16], frame.Sequence)
	_, err := w.Write(raw[:])
	return err
}
