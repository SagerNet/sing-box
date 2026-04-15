package tlsspoof

import (
	"net"

	E "github.com/sagernet/sing/common/exceptions"
)

type Method int

const (
	MethodWrongSequence Method = iota
	MethodWrongChecksum
)

const (
	MethodNameWrongSequence = "wrong-sequence"
	MethodNameWrongChecksum = "wrong-checksum"
)

func ParseMethod(s string) (Method, error) {
	switch s {
	case "", MethodNameWrongSequence:
		return MethodWrongSequence, nil
	case MethodNameWrongChecksum:
		return MethodWrongChecksum, nil
	default:
		return 0, E.New("tls_spoof: unknown method: ", s)
	}
}

func (m Method) String() string {
	switch m {
	case MethodWrongSequence:
		return MethodNameWrongSequence
	case MethodWrongChecksum:
		return MethodNameWrongChecksum
	default:
		return "unknown"
	}
}

type Spoofer interface {
	Inject(payload []byte) error
	Close() error
}

func NewSpoofer(conn net.Conn, method Method) (Spoofer, error) {
	return newRawSpoofer(conn, method)
}

type Conn struct {
	net.Conn
	spoofer  Spoofer
	fakeSNI  string
	injected bool
}

func NewConn(conn net.Conn, spoofer Spoofer, fakeSNI string) *Conn {
	return &Conn{
		Conn:    conn,
		spoofer: spoofer,
		fakeSNI: fakeSNI,
	}
}

func (c *Conn) Write(b []byte) (int, error) {
	if c.injected {
		return c.Conn.Write(b)
	}
	defer c.spoofer.Close()
	fake, err := rewriteSNI(b, c.fakeSNI)
	if err != nil {
		return 0, E.Cause(err, "tls_spoof: rewrite SNI")
	}
	err = c.spoofer.Inject(fake)
	if err != nil {
		return 0, E.Cause(err, "tls_spoof: inject")
	}
	c.injected = true
	return c.Conn.Write(b)
}

func (c *Conn) Close() error {
	return E.Append(c.Conn.Close(), c.spoofer.Close(), func(e error) error {
		return E.Cause(e, "close spoofer")
	})
}

func (c *Conn) ReaderReplaceable() bool {
	return true
}

func (c *Conn) WriterReplaceable() bool {
	return c.injected
}

func (c *Conn) Upstream() any {
	return c.Conn
}
