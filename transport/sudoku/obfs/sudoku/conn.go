package sudoku

import (
	"bufio"
	"bytes"
	crypto_rand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"net"
	"sync"
)

const IOBufferSize = 32 * 1024

var perm4 = [24][4]byte{
	{0, 1, 2, 3},
	{0, 1, 3, 2},
	{0, 2, 1, 3},
	{0, 2, 3, 1},
	{0, 3, 1, 2},
	{0, 3, 2, 1},
	{1, 0, 2, 3},
	{1, 0, 3, 2},
	{1, 2, 0, 3},
	{1, 2, 3, 0},
	{1, 3, 0, 2},
	{1, 3, 2, 0},
	{2, 0, 1, 3},
	{2, 0, 3, 1},
	{2, 1, 0, 3},
	{2, 1, 3, 0},
	{2, 3, 0, 1},
	{2, 3, 1, 0},
	{3, 0, 1, 2},
	{3, 0, 2, 1},
	{3, 1, 0, 2},
	{3, 1, 2, 0},
	{3, 2, 0, 1},
	{3, 2, 1, 0},
}

type Conn struct {
	net.Conn
	table      *Table
	reader     *bufio.Reader
	recorder   *bytes.Buffer
	recording  bool
	recordLock sync.Mutex

	rawBuf      []byte
	pendingData []byte
	hintBuf     []byte

	rng         *rand.Rand
	paddingRate float32
}

func NewConn(c net.Conn, table *Table, pMin, pMax int, record bool) *Conn {
	var seedBytes [8]byte
	if _, err := crypto_rand.Read(seedBytes[:]); err != nil {
		binary.BigEndian.PutUint64(seedBytes[:], uint64(rand.Int63()))
	}
	seed := int64(binary.BigEndian.Uint64(seedBytes[:]))
	localRng := rand.New(rand.NewSource(seed))

	min := float32(pMin) / 100.0
	span := float32(pMax-pMin) / 100.0
	rate := min + localRng.Float32()*span

	sc := &Conn{
		Conn:        c,
		table:       table,
		reader:      bufio.NewReaderSize(c, IOBufferSize),
		rawBuf:      make([]byte, IOBufferSize),
		pendingData: make([]byte, 0, 4096),
		hintBuf:     make([]byte, 0, 4),
		rng:         localRng,
		paddingRate: rate,
	}
	if record {
		sc.recorder = new(bytes.Buffer)
		sc.recording = true
	}
	return sc
}

func (c *Conn) StopRecording() {
	c.recordLock.Lock()
	c.recording = false
	c.recorder = nil
	c.recordLock.Unlock()
}

func (c *Conn) GetBufferedAndRecorded() []byte {
	if c == nil {
		return nil
	}

	c.recordLock.Lock()
	defer c.recordLock.Unlock()

	var recorded []byte
	if c.recorder != nil {
		recorded = c.recorder.Bytes()
	}

	buffered := c.reader.Buffered()
	if buffered > 0 {
		peeked, _ := c.reader.Peek(buffered)
		full := make([]byte, len(recorded)+len(peeked))
		copy(full, recorded)
		copy(full[len(recorded):], peeked)
		return full
	}
	return recorded
}

func (c *Conn) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	out := make([]byte, 0, len(p)*6)
	pads := c.table.PaddingPool
	padLen := len(pads)

	for _, b := range p {
		if padLen > 0 && c.rng.Float32() < c.paddingRate {
			out = append(out, pads[c.rng.Intn(padLen)])
		}

		puzzles := c.table.EncodeTable[b]
		puzzle := puzzles[c.rng.Intn(len(puzzles))]

		perm := perm4[c.rng.Intn(len(perm4))]
		for _, idx := range perm {
			if padLen > 0 && c.rng.Float32() < c.paddingRate {
				out = append(out, pads[c.rng.Intn(padLen)])
			}
			out = append(out, puzzle[idx])
		}
	}

	if padLen > 0 && c.rng.Float32() < c.paddingRate {
		out = append(out, pads[c.rng.Intn(padLen)])
	}

	_, err := c.Conn.Write(out)
	return len(p), err
}

func (c *Conn) Read(p []byte) (int, error) {
	if len(c.pendingData) > 0 {
		n := copy(p, c.pendingData)
		if n == len(c.pendingData) {
			c.pendingData = c.pendingData[:0]
		} else {
			c.pendingData = c.pendingData[n:]
		}
		return n, nil
	}

	for {
		if len(c.pendingData) > 0 {
			break
		}

		nr, rErr := c.reader.Read(c.rawBuf)
		if nr > 0 {
			chunk := c.rawBuf[:nr]
			c.recordLock.Lock()
			if c.recording {
				c.recorder.Write(chunk)
			}
			c.recordLock.Unlock()

			for _, b := range chunk {
				if !c.table.layout.isHint(b) {
					continue
				}

				c.hintBuf = append(c.hintBuf, b)
				if len(c.hintBuf) == 4 {
					key := packHintsToKey([4]byte{c.hintBuf[0], c.hintBuf[1], c.hintBuf[2], c.hintBuf[3]})
					val, ok := c.table.DecodeMap[key]
					if !ok {
						return 0, ErrInvalidSudokuMapMiss
					}
					c.pendingData = append(c.pendingData, val)
					c.hintBuf = c.hintBuf[:0]
				}
			}
		}

		if rErr != nil {
			return 0, rErr
		}
		if len(c.pendingData) > 0 {
			break
		}
	}

	n := copy(p, c.pendingData)
	if n == len(c.pendingData) {
		c.pendingData = c.pendingData[:0]
	} else {
		c.pendingData = c.pendingData[n:]
	}
	return n, nil
}

