package mtproto

import (
	"crypto/cipher"
	"encoding/binary"
	"io"
	"net"
	"sync"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/buf"
	E "github.com/sagernet/sing/common/exceptions"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/rw"
)

var (
	_ net.Conn         = (*FakeTLSConn)(nil)
	_ N.ExtendedWriter = (*FakeTLSConn)(nil)
)

type FakeTLSConn struct {
	net.Conn
	remain    int
	writeLock sync.Mutex

	clientEncryptor cipher.Stream
	clientDecryptor cipher.Stream

	serverEncryptor cipher.Stream
	serverDecryptor cipher.Stream

	unreadServerHandshake []byte
	serverHandshakeMutex  sync.Locker
}

func (c *FakeTLSConn) SetupObfs2(en, de cipher.Stream) {
	c.clientEncryptor = en
	c.clientDecryptor = de
	c.serverHandshakeMutex = &sync.Mutex{}
}

func (c *FakeTLSConn) read(p []byte) (n int, err error) {
	n, err = io.ReadFull(c.Conn, p)
	if err != nil {
		return
	}
	if c.clientDecryptor != nil {
		c.clientDecryptor.XORKeyStream(p, p[:n])
	}
	if c.serverEncryptor != nil {
		c.serverEncryptor.XORKeyStream(p, p[:n])
	}
	return
}

func (c *FakeTLSConn) Write(p []byte) (n int, err error) {
	lenP := len(p)
	frame := buf.Get(5 + lenP)
	frame[0] = TypeApplicationData
	frame[1] = 0x03
	frame[2] = 0x03
	binary.BigEndian.PutUint16(frame[3:], uint16(len(p)))
	if c.serverDecryptor != nil {
		c.serverDecryptor.XORKeyStream(frame[5:], p)
	}
	if c.clientEncryptor != nil {
		c.clientEncryptor.XORKeyStream(frame[5:], frame[5:5+lenP])
	}
	c.writeLock.Lock()
	_, err = c.Conn.Write(frame)
	c.writeLock.Unlock()
	buf.Put(frame)
	if err != nil {
		return 0, err
	}
	return len(p), nil
}

func (c *FakeTLSConn) WriteBuffer(buffer *buf.Buffer) error {
	if c.serverDecryptor != nil {
		c.serverDecryptor.XORKeyStream(buffer.Bytes(), buffer.Bytes())
	}
	if c.clientEncryptor != nil {
		c.clientEncryptor.XORKeyStream(buffer.Bytes(), buffer.Bytes())
	}
	l := buffer.Len()
	header := buffer.ExtendHeader(5)
	header[0] = TypeApplicationData
	header[1] = 0x03
	header[2] = 0x03
	binary.BigEndian.PutUint16(header[3:], uint16(l))
	return common.Error(c.Conn.Write(buffer.Bytes()))
}

func (c *FakeTLSConn) Read(p []byte) (n int, err error) {
	lenP := len(p)
	if c.serverEncryptor == nil && c.serverHandshakeMutex != nil {
		c.serverHandshakeMutex.Lock()
		defer c.serverHandshakeMutex.Unlock()
		if c.serverEncryptor == nil {
			en, de, h := GenerateObfs2ServerHandshake()
			lenH := len(h)
			if lenH < lenP {
				copy(p, h)
				c.serverEncryptor = en
				c.serverDecryptor = de
				return lenH, nil
			} else if lenH == lenP {
				copy(p, h)
				c.serverEncryptor = en
				c.serverDecryptor = de
				return lenH, nil
			} else { // lenH > lenP
				copy(p, h)
				c.unreadServerHandshake = h[lenP:]
				c.serverEncryptor = en
				c.serverDecryptor = de
				return lenP, nil
			}
		}
	}
	if lenH := len(c.unreadServerHandshake); lenH > 0 {
		if lenH < lenP {
			copy(p, c.unreadServerHandshake)
			p = p[lenH:]
		} else if lenH == lenP {
			copy(p, c.unreadServerHandshake)
			return lenH, nil
		} else { // lenH > lenP
			copy(p, c.unreadServerHandshake)
			c.unreadServerHandshake = c.unreadServerHandshake[lenP:]
			return lenP, nil
		}
	}

read:
	lenP = len(p)
	if c.remain > 0 {
		if c.remain >= lenP {
			n, err = c.read(p)
			c.remain -= n
			return n, err
		}
		n, err = c.read(p[:c.remain])
		if err != nil {
			return n, err
		}
		p = p[n:] //nolint:staticcheck
		c.remain -= n
		return n, nil
	}
	header := buf.Get(5)
	defer buf.Put(header)
	_, err = c.Conn.Read(header)
	if err != nil {
		return 0, err
	}
	l := int(binary.BigEndian.Uint16(header[3:]))
	switch header[0] {
	case TypeChangeCipherSpec:
		err = rw.SkipN(c.Conn, l)
		if err != nil {
			return 0, err
		}
		goto read
	case TypeApplicationData:
		if lenP > l {
			_n, err := c.read(p[:l])
			n += _n
			return n, err
		} else if lenP < l {
			_n, err := c.read(p)
			n += _n
			c.remain = l - _n
			return n, err
		}
		_n, err := c.read(p)
		n += _n
		return n, err
	}
	return n, E.New("unsupported record type: ", header[0])
}

func (c *FakeTLSConn) FrontHeadroom() int {
	return 5
}
