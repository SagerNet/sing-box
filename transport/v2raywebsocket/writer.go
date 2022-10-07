package v2raywebsocket

import (
	"encoding/binary"
	"math/rand"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/websocket"
)

type Writer struct {
	*websocket.Conn
	writer   N.ExtendedWriter
	isServer bool
}

func NewWriter(conn *websocket.Conn, isServer bool) *Writer {
	return &Writer{
		conn,
		bufio.NewExtendedWriter(conn.NetConn()),
		isServer,
	}
}

func (w *Writer) Write(p []byte) (n int, err error) {
	err = w.Conn.WriteMessage(websocket.BinaryMessage, p)
	if err != nil {
		return
	}
	return len(p), nil
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
	header[0] = websocket.BinaryMessage | 1<<7
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
		maskBytes(*(*[4]byte)(header[1+payloadBitLength:]), 0, data)
	}

	return w.writer.WriteBuffer(buffer)
}

func (w *Writer) Upstream() any {
	return w.Conn.NetConn()
}

func (w *Writer) FrontHeadroom() int {
	return 14
}
