package sudoku

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/transport/sudoku/crypto"
	"github.com/sagernet/sing-box/transport/sudoku/obfs/sudoku"
)

func pickClientTable(cfg *ProtocolConfig) (*sudoku.Table, byte, error) {
	candidates := cfg.tableCandidates()
	if len(candidates) == 0 {
		return nil, 0, fmt.Errorf("no table configured")
	}
	if len(candidates) == 1 {
		return candidates[0], 0, nil
	}
	idx := int(randomByte()) % len(candidates)
	return candidates[idx], byte(idx), nil
}

type readOnlyConn struct {
	*bytes.Reader
}

func (c *readOnlyConn) Write([]byte) (int, error)        { return 0, io.ErrClosedPipe }
func (c *readOnlyConn) Close() error                     { return nil }
func (c *readOnlyConn) LocalAddr() net.Addr              { return nil }
func (c *readOnlyConn) RemoteAddr() net.Addr             { return nil }
func (c *readOnlyConn) SetDeadline(time.Time) error      { return nil }
func (c *readOnlyConn) SetReadDeadline(time.Time) error  { return nil }
func (c *readOnlyConn) SetWriteDeadline(time.Time) error { return nil }

func drainBuffered(r *bufio.Reader) ([]byte, error) {
	n := r.Buffered()
	if n <= 0 {
		return nil, nil
	}
	out := make([]byte, n)
	_, err := io.ReadFull(r, out)
	return out, err
}

func probeHandshakeBytes(probe []byte, cfg *ProtocolConfig, table *sudoku.Table) error {
	rc := &readOnlyConn{Reader: bytes.NewReader(probe)}
	_, obfsConn := buildServerObfsConn(rc, cfg, table, false)
	cConn, err := crypto.NewAEADConn(obfsConn, cfg.Key, cfg.AEADMethod)
	if err != nil {
		return err
	}

	var handshakeBuf [16]byte
	if _, err := io.ReadFull(cConn, handshakeBuf[:]); err != nil {
		return err
	}
	ts := int64(binary.BigEndian.Uint64(handshakeBuf[:8]))
	if absInt64(time.Now().Unix()-ts) > 60 {
		return fmt.Errorf("timestamp skew/replay detected")
	}

	modeBuf := []byte{0}
	if _, err := io.ReadFull(cConn, modeBuf); err != nil {
		return err
	}
	if modeBuf[0] != downlinkMode(cfg) {
		return fmt.Errorf("downlink mode mismatch")
	}

	return nil
}

func selectTableByProbe(r *bufio.Reader, cfg *ProtocolConfig, tables []*sudoku.Table) (*sudoku.Table, []byte, error) {
	const (
		maxProbeBytes = 64 * 1024
		readChunk     = 4 * 1024
	)
	if len(tables) == 0 {
		return nil, nil, fmt.Errorf("no table candidates")
	}
	if len(tables) > 255 {
		return nil, nil, fmt.Errorf("too many table candidates: %d", len(tables))
	}

	probe, err := drainBuffered(r)
	if err != nil {
		return nil, nil, fmt.Errorf("drain buffered bytes failed: %w", err)
	}

	tmp := make([]byte, readChunk)
	for {
		if len(tables) == 1 {
			tail, err := drainBuffered(r)
			if err != nil {
				return nil, nil, fmt.Errorf("drain buffered bytes failed: %w", err)
			}
			probe = append(probe, tail...)
			return tables[0], probe, nil
		}

		needMore := false
		for _, table := range tables {
			err := probeHandshakeBytes(probe, cfg, table)
			if err == nil {
				tail, err := drainBuffered(r)
				if err != nil {
					return nil, nil, fmt.Errorf("drain buffered bytes failed: %w", err)
				}
				probe = append(probe, tail...)
				return table, probe, nil
			}
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				needMore = true
			}
		}

		if !needMore {
			return nil, probe, fmt.Errorf("handshake table selection failed")
		}
		if len(probe) >= maxProbeBytes {
			return nil, probe, fmt.Errorf("handshake probe exceeded %d bytes", maxProbeBytes)
		}

		n, err := r.Read(tmp)
		if n > 0 {
			probe = append(probe, tmp[:n]...)
		}
		if err != nil {
			return nil, probe, fmt.Errorf("handshake probe read failed: %w", err)
		}
	}
}

