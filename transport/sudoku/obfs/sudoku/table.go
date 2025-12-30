package sudoku

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math/rand"
	"time"
)

var ErrInvalidSudokuMapMiss = errors.New("INVALID_SUDOKU_MAP_MISS")

type Table struct {
	EncodeTable [256][][4]byte
	DecodeMap   map[uint32]byte
	PaddingPool []byte
	IsASCII     bool
	layout      *byteLayout
}

func NewTableWithCustom(key string, mode string, customPattern string) (*Table, error) {
	layout, err := resolveLayout(mode, customPattern)
	if err != nil {
		return nil, err
	}

	t := &Table{
		DecodeMap: make(map[uint32]byte),
		IsASCII:   layout.name == "ascii",
		layout:    layout,
	}
	t.PaddingPool = append(t.PaddingPool, layout.paddingPool...)

	allGrids := GenerateAllGrids()
	hash := sha256.Sum256([]byte(key))
	seed := int64(binary.BigEndian.Uint64(hash[:8]))
	rng := rand.New(rand.NewSource(seed))

	shuffledGrids := make([]Grid, 288)
	copy(shuffledGrids, allGrids)
	rng.Shuffle(len(shuffledGrids), func(i, j int) {
		shuffledGrids[i], shuffledGrids[j] = shuffledGrids[j], shuffledGrids[i]
	})

	// Precompute combinations of 4 positions out of 16.
	var combinations [][]int
	var combine func(int, int, []int)
	combine = func(start, k int, current []int) {
		if k == 0 {
			tmp := make([]int, len(current))
			copy(tmp, current)
			combinations = append(combinations, tmp)
			return
		}
		for i := start; i <= 16-k; i++ {
			current = append(current, i)
			combine(i+1, k-1, current)
			current = current[:len(current)-1]
		}
	}
	combine(0, 4, []int{})

	for byteVal := 0; byteVal < 256; byteVal++ {
		targetGrid := shuffledGrids[byteVal]
		for _, positions := range combinations {
			var rawParts [4]struct{ val, pos byte }
			for i, pos := range positions {
				val := targetGrid[pos] // 1..4
				rawParts[i] = struct{ val, pos byte }{val, uint8(pos)}
			}

			matchCount := 0
			for _, g := range allGrids {
				match := true
				for _, p := range rawParts {
					if g[p.pos] != p.val {
						match = false
						break
					}
				}
				if match {
					matchCount++
					if matchCount > 1 {
						break
					}
				}
			}
			if matchCount != 1 {
				continue
			}

			var currentHints [4]byte
			for i, p := range rawParts {
				currentHints[i] = t.layout.encodeHint(p.val-1, p.pos)
			}

			t.EncodeTable[byteVal] = append(t.EncodeTable[byteVal], currentHints)
			key := packHintsToKey(currentHints)
			t.DecodeMap[key] = byte(byteVal)
		}
	}

	_ = time.Now() // keep time import stable if callers add logging in the future
	return t, nil
}

func packHintsToKey(hints [4]byte) uint32 {
	// Sorting network for 4 elements.
	if hints[0] > hints[1] {
		hints[0], hints[1] = hints[1], hints[0]
	}
	if hints[2] > hints[3] {
		hints[2], hints[3] = hints[3], hints[2]
	}
	if hints[0] > hints[2] {
		hints[0], hints[2] = hints[2], hints[0]
	}
	if hints[1] > hints[3] {
		hints[1], hints[3] = hints[3], hints[1]
	}
	if hints[1] > hints[2] {
		hints[1], hints[2] = hints[2], hints[1]
	}

	return uint32(hints[0])<<24 | uint32(hints[1])<<16 | uint32(hints[2])<<8 | uint32(hints[3])
}

