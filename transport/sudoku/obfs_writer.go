package sudoku

import (
	crypto_rand "crypto/rand"
	"encoding/binary"
	"math/rand"
	"net"

	"github.com/sagernet/sing-box/transport/sudoku/obfs/sudoku"
)

// perm4 matches the upstream Sudoku obfuscation permutation set.
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

type sudokuObfsWriter struct {
	conn        net.Conn
	table       *sudoku.Table
	rng         *rand.Rand
	paddingRate float32

	outBuf []byte
	pads   []byte
	padLen int
}

func newSudokuObfsWriter(conn net.Conn, table *sudoku.Table, pMin, pMax int) *sudokuObfsWriter {
	var seedBytes [8]byte
	if _, err := crypto_rand.Read(seedBytes[:]); err != nil {
		binary.BigEndian.PutUint64(seedBytes[:], uint64(rand.Int63()))
	}
	seed := int64(binary.BigEndian.Uint64(seedBytes[:]))
	localRng := rand.New(rand.NewSource(seed))

	min := float32(pMin) / 100.0
	span := float32(pMax-pMin) / 100.0
	rate := min + localRng.Float32()*span

	w := &sudokuObfsWriter{
		conn:        conn,
		table:       table,
		rng:         localRng,
		paddingRate: rate,
	}
	w.pads = table.PaddingPool
	w.padLen = len(w.pads)
	return w
}

func (w *sudokuObfsWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// Worst-case: 4 hints + up to 6 paddings per input byte.
	needed := len(p)*10 + 1
	if cap(w.outBuf) < needed {
		w.outBuf = make([]byte, 0, needed)
	}
	out := w.outBuf[:0]

	pads := w.pads
	padLen := w.padLen

	for _, b := range p {
		if padLen > 0 && w.rng.Float32() < w.paddingRate {
			out = append(out, pads[w.rng.Intn(padLen)])
		}

		puzzles := w.table.EncodeTable[b]
		puzzle := puzzles[w.rng.Intn(len(puzzles))]

		perm := perm4[w.rng.Intn(len(perm4))]
		for _, idx := range perm {
			if padLen > 0 && w.rng.Float32() < w.paddingRate {
				out = append(out, pads[w.rng.Intn(padLen)])
			}
			out = append(out, puzzle[idx])
		}
	}

	if padLen > 0 && w.rng.Float32() < w.paddingRate {
		out = append(out, pads[w.rng.Intn(padLen)])
	}

	w.outBuf = out
	_, err := w.conn.Write(out)
	return len(p), err
}

