package usbip

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUSBIPSubmitCommandRoundTripOut(t *testing.T) {
	t.Parallel()

	expected := SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    7,
			DevID:     0x00030009,
			Direction: USBIPDirOut,
			Endpoint:  2,
		},
		TransferFlags:        0x400,
		TransferBufferLength: 3,
		StartFrame:           11,
		NumberOfPackets:      0,
		Interval:             4,
		Setup:                [8]byte{0, 1, 2, 3, 4, 5, 6, 7},
		Buffer:               []byte{1, 2, 3},
	}

	var buffer bytes.Buffer
	require.NoError(t, WriteSubmitCommand(&buffer, expected))

	header, err := ReadDataHeader(&buffer)
	require.NoError(t, err)
	require.Equal(t, expected.Header, header)

	actual, err := ReadSubmitCommandBody(&buffer, header)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestUSBIPSubmitCommandRoundTripInOmitsCommandPayload(t *testing.T) {
	t.Parallel()

	expected := SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    8,
			DevID:     0x00030009,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 4,
		Buffer:               []byte{9, 8, 7, 6},
	}

	var buffer bytes.Buffer
	require.NoError(t, WriteSubmitCommand(&buffer, expected))
	require.Equal(t, dataHeaderSize, buffer.Len())

	header, err := ReadDataHeader(&buffer)
	require.NoError(t, err)
	actual, err := ReadSubmitCommandBody(&buffer, header)
	require.NoError(t, err)
	require.Equal(t, expected.Header, actual.Header)
	require.Equal(t, expected.TransferBufferLength, actual.TransferBufferLength)
	require.Empty(t, actual.Buffer)
}

func TestUSBIPSubmitResponseRoundTripInWithIsoPackets(t *testing.T) {
	t.Parallel()

	expected := SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    9,
			DevID:     0x00030009,
			Direction: USBIPDirIn,
			Endpoint:  3,
		},
		Status:          0,
		ActualLength:    4,
		StartFrame:      21,
		NumberOfPackets: 2,
		ErrorCount:      1,
		Buffer:          []byte{9, 8, 7, 6},
		IsoPackets: []IsoPacketDescriptor{
			{Offset: 0, Length: 2, ActualLength: 2, Status: 0},
			{Offset: 2, Length: 2, ActualLength: 1, Status: -32},
		},
	}

	var buffer bytes.Buffer
	require.NoError(t, WriteSubmitResponse(&buffer, expected))

	header, err := ReadDataHeader(&buffer)
	require.NoError(t, err)
	require.Equal(t, expected.Header, header)

	actual, err := ReadSubmitResponseBody(&buffer, header)
	require.NoError(t, err)
	require.Equal(t, expected, actual)
}

func TestUSBIPSubmitResponseRoundTripOutOmitsResponsePayload(t *testing.T) {
	t.Parallel()

	expected := SubmitResponse{
		Header: DataHeader{
			Command:   RetSubmit,
			SeqNum:    10,
			DevID:     0x00030009,
			Direction: USBIPDirOut,
			Endpoint:  2,
		},
		Status:       0,
		ActualLength: 3,
		Buffer:       []byte{1, 2, 3},
	}

	var buffer bytes.Buffer
	require.NoError(t, WriteSubmitResponse(&buffer, expected))
	require.Equal(t, dataHeaderSize, buffer.Len())

	header, err := ReadDataHeader(&buffer)
	require.NoError(t, err)
	actual, err := ReadSubmitResponseBody(&buffer, header)
	require.NoError(t, err)
	require.Equal(t, expected.Header, actual.Header)
	require.Equal(t, expected.ActualLength, actual.ActualLength)
	require.Empty(t, actual.Buffer)
}

func TestUSBIPUnlinkRoundTrip(t *testing.T) {
	t.Parallel()

	command := UnlinkCommand{
		Header: DataHeader{
			Command:   CmdUnlink,
			SeqNum:    12,
			DevID:     0x00030009,
			Direction: USBIPDirOut,
			Endpoint:  0,
		},
		SeqNum: 11,
	}

	var buffer bytes.Buffer
	require.NoError(t, WriteUnlinkCommand(&buffer, command))

	header, err := ReadDataHeader(&buffer)
	require.NoError(t, err)
	actualCommand, err := ReadUnlinkCommandBody(&buffer, header)
	require.NoError(t, err)
	require.Equal(t, command, actualCommand)

	response := UnlinkResponse{
		Header: DataHeader{
			Command:   RetUnlink,
			SeqNum:    command.Header.SeqNum,
			DevID:     command.Header.DevID,
			Direction: command.Header.Direction,
			Endpoint:  command.Header.Endpoint,
		},
		Status: 0,
	}
	buffer.Reset()
	require.NoError(t, WriteUnlinkResponse(&buffer, response))

	header, err = ReadDataHeader(&buffer)
	require.NoError(t, err)
	actualResponse, err := ReadUnlinkResponseBody(&buffer, header)
	require.NoError(t, err)
	require.Equal(t, response, actualResponse)
}

func TestUSBIPUnlinkDelayedFakeTransfer(t *testing.T) {
	t.Parallel()

	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	deadline := time.Now().Add(5 * time.Second)
	require.NoError(t, client.SetDeadline(deadline))
	require.NoError(t, server.SetDeadline(deadline))

	serverDone := make(chan error, 1)
	go func() {
		header, err := ReadDataHeader(server)
		if err != nil {
			serverDone <- err
			return
		}
		command, err := ReadSubmitCommandBody(server, header)
		if err != nil {
			serverDone <- err
			return
		}

		header, err = ReadDataHeader(server)
		if err != nil {
			serverDone <- err
			return
		}
		unlink, err := ReadUnlinkCommandBody(server, header)
		if err != nil {
			serverDone <- err
			return
		}
		if unlink.SeqNum != command.Header.SeqNum {
			serverDone <- errors.New("unexpected unlink seq")
			return
		}
		serverDone <- WriteUnlinkResponse(server, UnlinkResponse{
			Header: DataHeader{
				Command:   RetUnlink,
				SeqNum:    unlink.Header.SeqNum,
				DevID:     unlink.Header.DevID,
				Direction: unlink.Header.Direction,
				Endpoint:  unlink.Header.Endpoint,
			},
			Status: 0,
		})
	}()

	require.NoError(t, WriteSubmitCommand(client, SubmitCommand{
		Header: DataHeader{
			Command:   CmdSubmit,
			SeqNum:    31,
			DevID:     0x00030009,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		TransferBufferLength: 64,
	}))
	require.NoError(t, WriteUnlinkCommand(client, UnlinkCommand{
		Header: DataHeader{
			Command:   CmdUnlink,
			SeqNum:    32,
			DevID:     0x00030009,
			Direction: USBIPDirIn,
			Endpoint:  1,
		},
		SeqNum: 31,
	}))

	header, err := ReadDataHeader(client)
	require.NoError(t, err)
	require.Equal(t, RetUnlink, header.Command)
	response, err := ReadUnlinkResponseBody(client, header)
	require.NoError(t, err)
	require.Equal(t, int32(0), response.Status)
	require.NoError(t, <-serverDone)
}

func TestUSBIPRejectsInvalidDataPlaneLengths(t *testing.T) {
	t.Parallel()

	var buffer bytes.Buffer
	err := WriteSubmitCommand(&buffer, SubmitCommand{
		Header:               DataHeader{Command: CmdSubmit, Direction: USBIPDirOut},
		TransferBufferLength: -1,
	})
	require.ErrorContains(t, err, "negative")

	err = WriteSubmitResponse(&buffer, SubmitResponse{
		Header:       DataHeader{Command: RetSubmit, Direction: USBIPDirIn},
		ActualLength: maxUSBIPTransferBufferLength + 1,
	})
	require.ErrorContains(t, err, "too large")

	var raw [28]byte
	binary.BigEndian.PutUint32(raw[4:8], 0xffffffff)
	_, err = ReadSubmitCommandBody(bytes.NewReader(raw[:]), DataHeader{Command: CmdSubmit, Direction: USBIPDirOut})
	require.ErrorContains(t, err, "negative")

	raw = [28]byte{}
	binary.BigEndian.PutUint32(raw[12:16], uint32(maxUSBIPIsoPackets+1))
	_, err = ReadSubmitCommandBody(bytes.NewReader(raw[:]), DataHeader{Command: CmdSubmit, Direction: USBIPDirIn})
	require.ErrorContains(t, err, "too large")
}
