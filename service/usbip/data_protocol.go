package usbip

import (
	"encoding/binary"
	"io"

	E "github.com/sagernet/sing/common/exceptions"
)

const (
	CmdSubmit uint32 = 0x00000001
	CmdUnlink uint32 = 0x00000002
	RetSubmit uint32 = 0x00000003
	RetUnlink uint32 = 0x00000004

	USBIPDirOut uint32 = 0
	USBIPDirIn  uint32 = 1

	dataHeaderSize               = 48
	unlinkBodySize               = 28
	isoPacketDescriptorWireSize  = 16
	maxUSBIPTransferBufferLength = 16 << 20
	maxUSBIPIsoPackets           = 4096
	nonIsoPacketCount            = -1
	usbipTransferFlagIsoASAP     = 0x0002
	usbipStatusECONNRESET        = -104
)

type DataHeader struct {
	Command   uint32
	SeqNum    uint32
	DevID     uint32
	Direction uint32
	Endpoint  uint32
}

type SubmitCommand struct {
	Header               DataHeader
	TransferFlags        int32
	TransferBufferLength int32
	StartFrame           int32
	NumberOfPackets      int32
	Interval             int32
	Setup                [8]byte
	Buffer               []byte
	IsoPackets           []IsoPacketDescriptor
}

type SubmitResponse struct {
	Header          DataHeader
	Status          int32
	ActualLength    int32
	StartFrame      int32
	NumberOfPackets int32
	ErrorCount      int32
	Setup           [8]byte
	Buffer          []byte
	IsoPackets      []IsoPacketDescriptor
}

type UnlinkCommand struct {
	Header DataHeader
	SeqNum uint32
}

type UnlinkResponse struct {
	Header DataHeader
	Status int32
}

type IsoPacketDescriptor struct {
	Offset       int32
	Length       int32
	ActualLength int32
	Status       int32
}

func ReadDataHeader(r io.Reader) (DataHeader, error) {
	var raw [20]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return DataHeader{}, err
	}
	return DataHeader{
		Command:   binary.BigEndian.Uint32(raw[0:4]),
		SeqNum:    binary.BigEndian.Uint32(raw[4:8]),
		DevID:     binary.BigEndian.Uint32(raw[8:12]),
		Direction: binary.BigEndian.Uint32(raw[12:16]),
		Endpoint:  binary.BigEndian.Uint32(raw[16:20]),
	}, nil
}

func ReadSubmitCommandBody(r io.Reader, header DataHeader) (SubmitCommand, error) {
	var raw [28]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return SubmitCommand{}, err
	}
	command := SubmitCommand{
		Header:               header,
		TransferFlags:        int32(binary.BigEndian.Uint32(raw[0:4])),
		TransferBufferLength: int32(binary.BigEndian.Uint32(raw[4:8])),
		StartFrame:           int32(binary.BigEndian.Uint32(raw[8:12])),
		NumberOfPackets:      int32(binary.BigEndian.Uint32(raw[12:16])),
		Interval:             int32(binary.BigEndian.Uint32(raw[16:20])),
	}
	copy(command.Setup[:], raw[20:28])
	buffer, isoPackets, err := readUSBIPPayload(r, header.Direction, command.TransferBufferLength, command.NumberOfPackets, true)
	if err != nil {
		return SubmitCommand{}, err
	}
	command.Buffer = buffer
	command.IsoPackets = isoPackets
	return command, nil
}

func ReadSubmitResponseBody(r io.Reader, header DataHeader, payloadDirection uint32) (SubmitResponse, error) {
	var raw [28]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return SubmitResponse{}, err
	}
	response := SubmitResponse{
		Header:          header,
		Status:          int32(binary.BigEndian.Uint32(raw[0:4])),
		ActualLength:    int32(binary.BigEndian.Uint32(raw[4:8])),
		StartFrame:      int32(binary.BigEndian.Uint32(raw[8:12])),
		NumberOfPackets: int32(binary.BigEndian.Uint32(raw[12:16])),
		ErrorCount:      int32(binary.BigEndian.Uint32(raw[16:20])),
	}
	copy(response.Setup[:], raw[20:28])
	bufferLength := response.ActualLength
	if bufferLength < 0 {
		bufferLength = 0
	}
	buffer, isoPackets, err := readUSBIPPayload(r, payloadDirection, bufferLength, response.NumberOfPackets, false)
	if err != nil {
		return SubmitResponse{}, err
	}
	response.Buffer = buffer
	response.IsoPackets = isoPackets
	return response, nil
}

func ReadUnlinkCommandBody(r io.Reader, header DataHeader) (UnlinkCommand, error) {
	var raw [unlinkBodySize]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return UnlinkCommand{}, err
	}
	return UnlinkCommand{
		Header: header,
		SeqNum: binary.BigEndian.Uint32(raw[0:4]),
	}, nil
}

func ReadUnlinkResponseBody(r io.Reader, header DataHeader) (UnlinkResponse, error) {
	var raw [unlinkBodySize]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		return UnlinkResponse{}, err
	}
	return UnlinkResponse{
		Header: header,
		Status: int32(binary.BigEndian.Uint32(raw[0:4])),
	}, nil
}

func WriteSubmitCommand(w io.Writer, command SubmitCommand) error {
	if err := validateUSBIPBufferLength(command.TransferBufferLength); err != nil {
		return err
	}
	packetCount := normalizeUSBIPIsoPacketCount(command.NumberOfPackets, command.IsoPackets)
	if err := validateUSBIPIsoPacketCount(packetCount); err != nil {
		return err
	}
	if err := writeDataHeader(w, command.Header); err != nil {
		return err
	}
	var raw [28]byte
	binary.BigEndian.PutUint32(raw[0:4], uint32(command.TransferFlags))
	binary.BigEndian.PutUint32(raw[4:8], uint32(command.TransferBufferLength))
	binary.BigEndian.PutUint32(raw[8:12], uint32(command.StartFrame))
	binary.BigEndian.PutUint32(raw[12:16], uint32(packetCount))
	binary.BigEndian.PutUint32(raw[16:20], uint32(command.Interval))
	copy(raw[20:28], command.Setup[:])
	if _, err := w.Write(raw[:]); err != nil {
		return err
	}
	return writeUSBIPPayload(w, command.Header.Direction, command.Buffer, command.IsoPackets, true)
}

func WriteSubmitResponse(w io.Writer, response SubmitResponse) error {
	if response.ActualLength < 0 {
		response.ActualLength = 0
	}
	if err := validateUSBIPBufferLength(response.ActualLength); err != nil {
		return err
	}
	packetCount := normalizeUSBIPIsoPacketCount(response.NumberOfPackets, response.IsoPackets)
	if err := validateUSBIPIsoPacketCount(packetCount); err != nil {
		return err
	}
	payloadDirection := response.Header.Direction
	header := responseDataHeader(response.Header)
	if err := writeDataHeader(w, header); err != nil {
		return err
	}
	var raw [28]byte
	binary.BigEndian.PutUint32(raw[0:4], uint32(response.Status))
	binary.BigEndian.PutUint32(raw[4:8], uint32(response.ActualLength))
	binary.BigEndian.PutUint32(raw[8:12], uint32(response.StartFrame))
	binary.BigEndian.PutUint32(raw[12:16], uint32(packetCount))
	binary.BigEndian.PutUint32(raw[16:20], uint32(response.ErrorCount))
	copy(raw[20:28], response.Setup[:])
	if _, err := w.Write(raw[:]); err != nil {
		return err
	}
	return writeUSBIPPayload(w, payloadDirection, response.Buffer, response.IsoPackets, false)
}

func WriteUnlinkCommand(w io.Writer, command UnlinkCommand) error {
	if err := writeDataHeader(w, command.Header); err != nil {
		return err
	}
	var raw [unlinkBodySize]byte
	binary.BigEndian.PutUint32(raw[0:4], command.SeqNum)
	_, err := w.Write(raw[:])
	return err
}

func WriteUnlinkResponse(w io.Writer, response UnlinkResponse) error {
	if err := writeDataHeader(w, responseDataHeader(response.Header)); err != nil {
		return err
	}
	var raw [unlinkBodySize]byte
	binary.BigEndian.PutUint32(raw[0:4], uint32(response.Status))
	_, err := w.Write(raw[:])
	return err
}

func writeDataHeader(w io.Writer, header DataHeader) error {
	var raw [20]byte
	binary.BigEndian.PutUint32(raw[0:4], header.Command)
	binary.BigEndian.PutUint32(raw[4:8], header.SeqNum)
	binary.BigEndian.PutUint32(raw[8:12], header.DevID)
	binary.BigEndian.PutUint32(raw[12:16], header.Direction)
	binary.BigEndian.PutUint32(raw[16:20], header.Endpoint)
	_, err := w.Write(raw[:])
	return err
}

func readUSBIPPayload(r io.Reader, direction uint32, bufferLength int32, packetCount int32, command bool) ([]byte, []IsoPacketDescriptor, error) {
	if err := validateUSBIPBufferLength(bufferLength); err != nil {
		return nil, nil, err
	}
	if err := validateUSBIPIsoPacketCount(packetCount); err != nil {
		return nil, nil, err
	}
	bufferSize := int(bufferLength)
	var buffer []byte
	if shouldCarryUSBIPBuffer(direction, command) && bufferSize > 0 {
		buffer = make([]byte, bufferSize)
		if _, err := io.ReadFull(r, buffer); err != nil {
			return nil, nil, err
		}
	}
	isoPackets, err := readUSBIPIsoPackets(r, packetCount)
	if err != nil {
		return nil, nil, err
	}
	return buffer, isoPackets, nil
}

func writeUSBIPPayload(w io.Writer, direction uint32, buffer []byte, isoPackets []IsoPacketDescriptor, command bool) error {
	if shouldCarryUSBIPBuffer(direction, command) && len(buffer) > 0 {
		if _, err := w.Write(buffer); err != nil {
			return err
		}
	}
	return writeUSBIPIsoPackets(w, isoPackets)
}

func shouldCarryUSBIPBuffer(direction uint32, command bool) bool {
	if command {
		return direction == USBIPDirOut
	}
	return direction == USBIPDirIn
}

func normalizeUSBIPIsoPacketCount(count int32, packets []IsoPacketDescriptor) int32 {
	if count == 0 && len(packets) == 0 {
		return nonIsoPacketCount
	}
	if count == 0 && len(packets) > 0 {
		return int32(len(packets))
	}
	return count
}

func responseDataHeader(header DataHeader) DataHeader {
	return DataHeader{
		Command: header.Command,
		SeqNum:  header.SeqNum,
	}
}

func readUSBIPIsoPackets(r io.Reader, count int32) ([]IsoPacketDescriptor, error) {
	if count <= 0 {
		return nil, nil
	}
	packets := make([]IsoPacketDescriptor, int(count))
	var raw [isoPacketDescriptorWireSize]byte
	for i := range packets {
		if _, err := io.ReadFull(r, raw[:]); err != nil {
			return nil, err
		}
		packets[i] = IsoPacketDescriptor{
			Offset:       int32(binary.BigEndian.Uint32(raw[0:4])),
			Length:       int32(binary.BigEndian.Uint32(raw[4:8])),
			ActualLength: int32(binary.BigEndian.Uint32(raw[8:12])),
			Status:       int32(binary.BigEndian.Uint32(raw[12:16])),
		}
	}
	return packets, nil
}

func writeUSBIPIsoPackets(w io.Writer, packets []IsoPacketDescriptor) error {
	var raw [isoPacketDescriptorWireSize]byte
	for i := range packets {
		binary.BigEndian.PutUint32(raw[0:4], uint32(packets[i].Offset))
		binary.BigEndian.PutUint32(raw[4:8], uint32(packets[i].Length))
		binary.BigEndian.PutUint32(raw[8:12], uint32(packets[i].ActualLength))
		binary.BigEndian.PutUint32(raw[12:16], uint32(packets[i].Status))
		if _, err := w.Write(raw[:]); err != nil {
			return err
		}
	}
	return nil
}

func validateUSBIPBufferLength(length int32) error {
	if length < 0 {
		return E.New("USB/IP transfer buffer length is negative: ", length)
	}
	if length > maxUSBIPTransferBufferLength {
		return E.New("USB/IP transfer buffer length too large: ", length)
	}
	return nil
}

func validateUSBIPIsoPacketCount(count int32) error {
	if count < nonIsoPacketCount {
		return E.New("USB/IP iso packet count is negative: ", count)
	}
	if count > maxUSBIPIsoPackets {
		return E.New("USB/IP iso packet count too large: ", count)
	}
	return nil
}
