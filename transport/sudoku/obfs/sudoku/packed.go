package sudoku

import (
	"bufio"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"io"
	"math/rand"
	"net"
	"sync"
)

// PackedConn is the bandwidth-optimized downlink mode.
// It encodes payload into 6-bit groups and maps them into Sudoku hints.
type PackedConn struct {
	net.Conn
	table  *Table
	reader *bufio.Reader

	rawBuf      []byte
	pendingData []byte

	writeMu  sync.Mutex
	writeBuf []byte
	bitBuf   uint64
	bitCount int

	readBitBuf uint64
	readBits   int

	rng         *rand.Rand
	paddingRate float32
	padMarker   byte
	padPool     []byte
}

func NewPackedConn(c net.Conn, table *Table, pMin, pMax int) *PackedConn {
	var seedBytes [8]byte
	if _, err := crypto_rand.Read(seedBytes[:]); err != nil {
		binary.BigEndian.PutUint64(seedBytes[:], uint64(rand.Int63()))
	}
	seed := int64(binary.BigEndian.Uint64(seedBytes[:]))
	localRng := rand.New(rand.NewSource(seed))

	min := float32(pMin) / 100.0
	span := float32(pMax-pMin) / 100.0
	rate := min + localRng.Float32()*span

	pc := &PackedConn{
		Conn:        c,
		table:       table,
		reader:      bufio.NewReaderSize(c, IOBufferSize),
		rawBuf:      make([]byte, IOBufferSize),
		pendingData: make([]byte, 0, 4096),
		writeBuf:    make([]byte, 0, 4096),
		rng:         localRng,
		paddingRate: rate,
	}

	pc.padMarker = table.layout.padMarker
	for _, b := range table.PaddingPool {
		if b != pc.padMarker {
			pc.padPool = append(pc.padPool, b)
		}
	}
	if len(pc.padPool) == 0 {
		pc.padPool = append(pc.padPool, pc.padMarker)
	}
	return pc
}

func (pc *PackedConn) maybeAddPadding(out []byte) []byte {
	if pc.rng.Float32() < pc.paddingRate {
		out = append(out, pc.padPool[pc.rng.Intn(len(pc.padPool))])
	}
	return out
}

func (pc *PackedConn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	pc.writeMu.Lock()
	defer pc.writeMu.Unlock()

	needed := len(p)*3/2 + 32
	if cap(pc.writeBuf) < needed {
		pc.writeBuf = make([]byte, 0, needed)
	}
	out := pc.writeBuf[:0]

	i := 0
	n := len(p)

	for pc.bitCount > 0 && i < n {
		out = pc.maybeAddPadding(out)
		b := p[i]
		i++
		pc.bitBuf = (pc.bitBuf << 8) | uint64(b)
		pc.bitCount += 8
		for pc.bitCount >= 6 {
			pc.bitCount -= 6
			group := byte(pc.bitBuf >> pc.bitCount)
			if pc.bitCount == 0 {
				pc.bitBuf = 0
			} else {
				pc.bitBuf &= (1 << pc.bitCount) - 1
			}
			out = pc.maybeAddPadding(out)
			out = append(out, pc.table.layout.encodeGroup(group&0x3F))
		}
	}

	for i+11 < n {
		for batch := 0; batch < 4; batch++ {
			b1, b2, b3 := p[i], p[i+1], p[i+2]
			i += 3

			g1 := (b1 >> 2) & 0x3F
			g2 := ((b1 & 0x03) << 4) | ((b2 >> 4) & 0x0F)
			g3 := ((b2 & 0x0F) << 2) | ((b3 >> 6) & 0x03)
			g4 := b3 & 0x3F

			out = pc.maybeAddPadding(out)
			out = append(out, pc.table.layout.encodeGroup(g1))
			out = pc.maybeAddPadding(out)
			out = append(out, pc.table.layout.encodeGroup(g2))
			out = pc.maybeAddPadding(out)
			out = append(out, pc.table.layout.encodeGroup(g3))
			out = pc.maybeAddPadding(out)
			out = append(out, pc.table.layout.encodeGroup(g4))
		}
	}

	for i+2 < n {
		b1, b2, b3 := p[i], p[i+1], p[i+2]
		i += 3

		g1 := (b1 >> 2) & 0x3F
		g2 := ((b1 & 0x03) << 4) | ((b2 >> 4) & 0x0F)
		g3 := ((b2 & 0x0F) << 2) | ((b3 >> 6) & 0x03)
		g4 := b3 & 0x3F

		out = pc.maybeAddPadding(out)
		out = append(out, pc.table.layout.encodeGroup(g1))
		out = pc.maybeAddPadding(out)
		out = append(out, pc.table.layout.encodeGroup(g2))
		out = pc.maybeAddPadding(out)
		out = append(out, pc.table.layout.encodeGroup(g3))
		out = pc.maybeAddPadding(out)
		out = append(out, pc.table.layout.encodeGroup(g4))
	}

	for ; i < n; i++ {
		b := p[i]
		pc.bitBuf = (pc.bitBuf << 8) | uint64(b)
		pc.bitCount += 8
		for pc.bitCount >= 6 {
			pc.bitCount -= 6
			group := byte(pc.bitBuf >> pc.bitCount)
			if pc.bitCount == 0 {
				pc.bitBuf = 0
			} else {
				pc.bitBuf &= (1 << pc.bitCount) - 1
			}
			out = pc.maybeAddPadding(out)
			out = append(out, pc.table.layout.encodeGroup(group&0x3F))
		}
	}

	if pc.bitCount > 0 {
		out = pc.maybeAddPadding(out)
		group := byte(pc.bitBuf << (6 - pc.bitCount))
		pc.bitBuf = 0
		pc.bitCount = 0
		out = append(out, pc.table.layout.encodeGroup(group&0x3F))
		out = append(out, pc.padMarker)
	}

	out = pc.maybeAddPadding(out)

	if len(out) > 0 {
		_, err := pc.Conn.Write(out)
		pc.writeBuf = out[:0]
		return len(p), err
	}
	pc.writeBuf = out[:0]
	return len(p), nil
}

func (pc *PackedConn) Flush() error {
	pc.writeMu.Lock()
	defer pc.writeMu.Unlock()

	out := pc.writeBuf[:0]
	if pc.bitCount > 0 {
		group := byte(pc.bitBuf << (6 - pc.bitCount))
		pc.bitBuf = 0
		pc.bitCount = 0

		out = append(out, pc.table.layout.encodeGroup(group&0x3F))
		out = append(out, pc.padMarker)
	}

	out = pc.maybeAddPadding(out)

	if len(out) > 0 {
		_, err := pc.Conn.Write(out)
		pc.writeBuf = out[:0]
		return err
	}
	return nil
}

func (pc *PackedConn) Read(p []byte) (int, error) {
	if len(pc.pendingData) > 0 {
		n := copy(p, pc.pendingData)
		if n == len(pc.pendingData) {
			pc.pendingData = pc.pendingData[:0]
		} else {
			remaining := len(pc.pendingData) - n
			copy(pc.pendingData, pc.pendingData[n:])
			pc.pendingData = pc.pendingData[:remaining]
		}
		return n, nil
	}

	for {
		nr, rErr := pc.reader.Read(pc.rawBuf)
		if nr > 0 {
			rBuf := pc.readBitBuf
			rBits := pc.readBits
			padMarker := pc.padMarker
			layout := pc.table.layout

			for _, b := range pc.rawBuf[:nr] {
				if !layout.isHint(b) {
					if b == padMarker {
						rBuf = 0
						rBits = 0
					}
					continue
				}

				group, ok := layout.decodeGroup(b)
				if !ok {
					return 0, ErrInvalidSudokuMapMiss
				}

				rBuf = (rBuf << 6) | uint64(group)
				rBits += 6

				if rBits >= 8 {
					rBits -= 8
					val := byte(rBuf >> rBits)
					pc.pendingData = append(pc.pendingData, val)
				}
			}

			pc.readBitBuf = rBuf
			pc.readBits = rBits
		}

		if rErr != nil {
			if rErr == io.EOF {
				pc.readBitBuf = 0
				pc.readBits = 0
			}
			if len(pc.pendingData) > 0 {
				break
			}
			return 0, rErr
		}

		if len(pc.pendingData) > 0 {
			break
		}
	}

	n := copy(p, pc.pendingData)
	if n == len(pc.pendingData) {
		pc.pendingData = pc.pendingData[:0]
	} else {
		remaining := len(pc.pendingData) - n
		copy(pc.pendingData, pc.pendingData[n:])
		pc.pendingData = pc.pendingData[:remaining]
	}
	return n, nil
}

