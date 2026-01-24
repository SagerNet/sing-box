package v2raywebsocket

import (
	"encoding/binary"
	"io"
	"math/rand"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/ws"
)

type Writer struct {
	writer   N.ExtendedWriter
	isServer bool
}

func NewWriter(writer io.Writer, state ws.State) *Writer {
	return &Writer{
		bufio.NewExtendedWriter(writer),
		state == ws.StateServerSide,
	}
}

func (w *Writer) WriteBuffer(buffer *buf.Buffer) error {
	var payloadBitLength int
	dataLen := buffer.Len()
	data := buffer.Bytes()
	if dataLen < 126 {
		payloadBitLength = 1
	} else if dataLen < 65536 {
		payloadBitLength = 3
	} else {
		payloadBitLength = 9
	}

	var headerLen int
	headerLen += 1 // FIN / RSV / OPCODE
	headerLen += payloadBitLength
	if !w.isServer {
		headerLen += 4 // MASK KEY
	}

	header := buffer.ExtendHeader(headerLen)
	header[0] = byte(ws.OpBinary) | 0x80
	if w.isServer {
		header[1] = 0
	} else {
		header[1] = 1 << 7
	}

	if dataLen < 126 {
		header[1] |= byte(dataLen)
	} else if dataLen < 65536 {
		header[1] |= 126
		binary.BigEndian.PutUint16(header[2:], uint16(dataLen))
	} else {
		header[1] |= 127
		binary.BigEndian.PutUint64(header[2:], uint64(dataLen))
	}

	if !w.isServer {
		maskKey := rand.Uint32()
		binary.BigEndian.PutUint32(header[1+payloadBitLength:], maskKey)
		ws.Cipher(data, [4]byte(header[1+payloadBitLength:]), 0)
	}

	return wrapWsError(w.writer.WriteBuffer(buffer))
}

func (w *Writer) FrontHeadroom() int {
	return 14
}
