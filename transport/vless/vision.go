package vless

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	"io"
	"math/big"
	"net"
	"reflect"
	"time"
	"unsafe"

	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
)

var tlsRegistry []func(conn net.Conn) (loaded bool, netConn net.Conn, reflectType reflect.Type, reflectPointer uintptr)

func init() {
	tlsRegistry = append(tlsRegistry, func(conn net.Conn) (loaded bool, netConn net.Conn, reflectType reflect.Type, reflectPointer uintptr) {
		tlsConn, loaded := conn.(*tls.Conn)
		if !loaded {
			return
		}
		return true, tlsConn.NetConn(), reflect.TypeOf(tlsConn).Elem(), uintptr(unsafe.Pointer(tlsConn))
	})
}

type VisionConn struct {
	net.Conn
	writer   N.VectorisedWriter
	input    *bytes.Reader
	rawInput *bytes.Buffer
	netConn  net.Conn

	userUUID                 [16]byte
	isTLS                    bool
	numberOfPacketToFilter   int
	isTLS12orAbove           bool
	remainingServerHello     int32
	cipher                   uint16
	enableXTLS               bool
	filterTlsApplicationData bool
	directWrite              bool
	writeUUID                bool
	filterUUID               bool
	remainingContent         int
	remainingPadding         int
	currentCommand           int
	directRead               bool
	remainingReader          io.Reader
}

func NewVisionConn(conn net.Conn, userUUID [16]byte) (*VisionConn, error) {
	var (
		loaded         bool
		reflectType    reflect.Type
		reflectPointer uintptr
		netConn        net.Conn
	)
	for _, tlsCreator := range tlsRegistry {
		loaded, netConn, reflectType, reflectPointer = tlsCreator(conn)
		if loaded {
			break
		}
	}
	if !loaded {
		return nil, C.ErrTLSRequired
	}
	input, _ := reflectType.FieldByName("input")
	rawInput, _ := reflectType.FieldByName("rawInput")
	return &VisionConn{
		Conn:                     conn,
		writer:                   bufio.NewVectorisedWriter(conn),
		input:                    (*bytes.Reader)(unsafe.Pointer(reflectPointer + input.Offset)),
		rawInput:                 (*bytes.Buffer)(unsafe.Pointer(reflectPointer + rawInput.Offset)),
		netConn:                  netConn,
		userUUID:                 userUUID,
		numberOfPacketToFilter:   8,
		remainingServerHello:     -1,
		filterTlsApplicationData: true,
		writeUUID:                true,
		filterUUID:               true,
		remainingContent:         -1,
		remainingPadding:         -1,
	}, nil
}

func (c *VisionConn) Read(p []byte) (n int, err error) {
	if c.remainingReader != nil {
		n, err = c.remainingReader.Read(p)
		if err == io.EOF {
			c.remainingReader = nil
			if n > 0 {
				return
			}
		}
	}
	if c.directRead {
		return c.netConn.Read(p)
	}
	n, err = c.Conn.Read(p)
	if err != nil {
		return
	}
	buffer := p[:n]
	if c.filterUUID && (c.isTLS || c.numberOfPacketToFilter > 0) {
		buffers := c.unPadding(buffer)
		if c.remainingContent == 0 && c.remainingPadding == 0 {
			if c.currentCommand == 1 {
				c.filterUUID = false
			} else if c.currentCommand == 2 {
				c.filterUUID = false
				c.directRead = true

				inputBuffer, err := io.ReadAll(c.input)
				if err != nil {
					return 0, err
				}
				buffers = append(buffers, inputBuffer)

				rawInputBuffer, err := io.ReadAll(c.rawInput)
				if err != nil {
					return 0, err
				}

				buffers = append(buffers, rawInputBuffer)
			} else if c.currentCommand != 0 {
				return 0, E.New("unknown command ", c.currentCommand)
			}
		}
		if c.numberOfPacketToFilter > 0 {
			c.filterTLS(buffers)
		}
		c.remainingReader = io.MultiReader(common.Map(buffers, func(it []byte) io.Reader { return bytes.NewReader(it) })...)
		return c.remainingReader.Read(p)
	} else {
		if c.numberOfPacketToFilter > 0 {
			c.filterTLS([][]byte{buffer})
		}
		return
	}
}

func (c *VisionConn) Write(p []byte) (n int, err error) {
	if c.numberOfPacketToFilter > 0 {
		c.filterTLS([][]byte{p})
	}
	if c.isTLS && c.filterTlsApplicationData {
		inputLen := len(p)
		buffers := reshapeBuffer(p)
		var specIndex int
		for i, buffer := range buffers {
			if buffer.Len() > 6 && bytes.Equal(tlsApplicationDataStart, buffer.To(3)) {
				var command byte = 1
				if c.enableXTLS {
					c.directWrite = true
					specIndex = i
					command = 2
				}
				c.filterTlsApplicationData = false
				buffers[i] = c.padding(buffer, command)
				break
			} else if !c.isTLS12orAbove && c.numberOfPacketToFilter == 0 {
				c.filterTlsApplicationData = false
				buffers[i] = c.padding(buffer, 0x01)
				break
			}
			buffers[i] = c.padding(buffer, 0x00)
		}
		if c.directWrite {
			encryptedBuffer := buffers[:specIndex+1]
			err = c.writer.WriteVectorised(encryptedBuffer)
			if err != nil {
				return
			}
			buffers = buffers[specIndex+1:]
			c.writer = bufio.NewVectorisedWriter(c.netConn)
			time.Sleep(5 * time.Millisecond) // wtf
		}
		err = c.writer.WriteVectorised(buffers)
		if err == nil {
			n = inputLen
		}
		return
	}
	if c.directWrite {
		return c.netConn.Write(p)
	} else {
		return c.Conn.Write(p)
	}
}

func (c *VisionConn) filterTLS(buffers [][]byte) {
	for _, buffer := range buffers {
		c.numberOfPacketToFilter--
		if len(buffer) > 6 {
			if buffer[0] == 22 && buffer[1] == 3 && buffer[2] == 3 {
				c.isTLS = true
				if buffer[5] == 2 {
					c.isTLS12orAbove = true
					c.remainingServerHello = (int32(buffer[3])<<8 | int32(buffer[4])) + 5
					if len(buffer) >= 79 && c.remainingServerHello >= 79 {
						sessionIdLen := int32(buffer[43])
						cipherSuite := buffer[43+sessionIdLen+1 : 43+sessionIdLen+3]
						c.cipher = uint16(cipherSuite[0])<<8 | uint16(cipherSuite[1])
					}
				}
			} else if bytes.Equal(tlsClientHandShakeStart, buffer[:2]) && buffer[5] == 1 {
				c.isTLS = true
			}
		}
		if c.remainingServerHello > 0 {
			end := int(c.remainingServerHello)
			if end > len(buffer) {
				end = len(buffer)
			}
			c.remainingServerHello -= int32(end)
			if bytes.Contains(buffer[:end], tls13SupportedVersions) {
				cipher, ok := tls13CipherSuiteDic[c.cipher]
				if ok && cipher != "TLS_AES_128_CCM_8_SHA256" {
					c.enableXTLS = true
				}
				c.numberOfPacketToFilter = 0
				return
			} else if c.remainingServerHello == 0 {
				c.numberOfPacketToFilter = 0
				return
			}
		}
	}
}

func (c *VisionConn) padding(buffer *buf.Buffer, command byte) *buf.Buffer {
	contentLen := 0
	paddingLen := 0
	if buffer != nil {
		contentLen = buffer.Len()
	}
	if contentLen < 900 {
		l, _ := rand.Int(rand.Reader, big.NewInt(500))
		paddingLen = int(l.Int64()) + 900 - contentLen
	}
	newBuffer := buf.New()
	if c.writeUUID {
		newBuffer.Write(c.userUUID[:])
		c.writeUUID = false
	}
	newBuffer.Write([]byte{command, byte(contentLen >> 8), byte(contentLen), byte(paddingLen >> 8), byte(paddingLen)})
	if buffer != nil {
		newBuffer.Write(buffer.Bytes())
		buffer.Release()
	}
	newBuffer.Extend(paddingLen)
	return newBuffer
}

func (c *VisionConn) unPadding(buffer []byte) [][]byte {
	var bufferIndex int
	if c.remainingContent == -1 && c.remainingPadding == -1 {
		if len(buffer) >= 21 && bytes.Equal(c.userUUID[:], buffer[:16]) {
			bufferIndex = 16
			c.remainingContent = 0
			c.remainingPadding = 0
		}
	}
	if c.remainingContent == -1 && c.remainingPadding == -1 {
		return [][]byte{buffer}
	}
	var buffers [][]byte
	for bufferIndex < len(buffer) {
		if c.remainingContent <= 0 && c.remainingPadding <= 0 {
			if c.currentCommand == 1 {
				buffers = append(buffers, buffer[bufferIndex:])
				break
			} else {
				paddingInfo := buffer[bufferIndex : bufferIndex+5]
				c.currentCommand = int(paddingInfo[0])
				c.remainingContent = int(paddingInfo[1])<<8 | int(paddingInfo[2])
				c.remainingPadding = int(paddingInfo[3])<<8 | int(paddingInfo[4])
				bufferIndex += 5
			}
		} else if c.remainingContent > 0 {
			end := c.remainingContent
			if end > len(buffer)-bufferIndex {
				end = len(buffer) - bufferIndex
			}
			buffers = append(buffers, buffer[bufferIndex:bufferIndex+end])
			c.remainingContent -= end
			bufferIndex += end
		} else {
			end := c.remainingPadding
			if end > len(buffer)-bufferIndex {
				end = len(buffer) - bufferIndex
			}
			c.remainingPadding -= end
			bufferIndex += end
		}
		if bufferIndex == len(buffer) {
			break
		}
	}
	return buffers
}
