package domain

import (
	"math/bits"
)

const prefixLabel = '\r'

// mod from https://github.com/openacid/succinct

type succinctSet struct {
	leaves, labelBitmap []uint64
	labels              []byte
	ranks, selects      []int32
}

func newSuccinctSet(keys []string) *succinctSet {
	ss := &succinctSet{}
	lIdx := 0
	type qElt struct{ s, e, col int }
	queue := []qElt{{0, len(keys), 0}}
	for i := 0; i < len(queue); i++ {
		elt := queue[i]
		if elt.col == len(keys[elt.s]) {
			// a leaf node
			elt.s++
			setBit(&ss.leaves, i, 1)
		}
		for j := elt.s; j < elt.e; {
			frm := j
			for ; j < elt.e && keys[j][elt.col] == keys[frm][elt.col]; j++ {
			}
			queue = append(queue, qElt{frm, j, elt.col + 1})
			ss.labels = append(ss.labels, keys[frm][elt.col])
			setBit(&ss.labelBitmap, lIdx, 0)
			lIdx++
		}
		setBit(&ss.labelBitmap, lIdx, 1)
		lIdx++
	}
	ss.init()
	return ss
}

func (ss *succinctSet) Has(key string) bool {
	var nodeId, bmIdx int
	for i := 0; i < len(key); i++ {
		currentChar := key[i]
		for ; ; bmIdx++ {
			if getBit(ss.labelBitmap, bmIdx) != 0 {
				return false
			}
			nextLabel := ss.labels[bmIdx-nodeId]
			if nextLabel == prefixLabel {
				return true
			}
			if nextLabel == currentChar {
				break
			}
		}
		nodeId = countZeros(ss.labelBitmap, ss.ranks, bmIdx+1)
		bmIdx = selectIthOne(ss.labelBitmap, ss.ranks, ss.selects, nodeId-1) + 1
	}
	if getBit(ss.leaves, nodeId) != 0 {
		return true
	}
	for ; ; bmIdx++ {
		if getBit(ss.labelBitmap, bmIdx) != 0 {
			return false
		}
		if ss.labels[bmIdx-nodeId] == prefixLabel {
			return true
		}
	}
}

func setBit(bm *[]uint64, i int, v int) {
	for i>>6 >= len(*bm) {
		*bm = append(*bm, 0)
	}
	(*bm)[i>>6] |= uint64(v) << uint(i&63)
}

func getBit(bm []uint64, i int) uint64 {
	return bm[i>>6] & (1 << uint(i&63))
}

func (ss *succinctSet) init() {
	ss.selects, ss.ranks = indexSelect32R64(ss.labelBitmap)
}

func countZeros(bm []uint64, ranks []int32, i int) int {
	a, _ := rank64(bm, ranks, int32(i))
	return i - int(a)
}

func selectIthOne(bm []uint64, ranks, selects []int32, i int) int {
	a, _ := select32R64(bm, selects, ranks, int32(i))
	return int(a)
}

func rank64(words []uint64, rindex []int32, i int32) (int32, int32) {
	wordI := i >> 6
	j := uint32(i & 63)
	n := rindex[wordI]
	w := words[wordI]
	c1 := n + int32(bits.OnesCount64(w&mask[j]))
	return c1, int32(w>>uint(j)) & 1
}

func indexRank64(words []uint64, opts ...bool) []int32 {
	trailing := false
	if len(opts) > 0 {
		trailing = opts[0]
	}
	l := len(words)
	if trailing {
		l++
	}
	idx := make([]int32, l)
	n := int32(0)
	for i := 0; i < len(words); i++ {
		idx[i] = n
		n += int32(bits.OnesCount64(words[i]))
	}
	if trailing {
		idx[len(words)] = n
	}
	return idx
}

func select32R64(words []uint64, selectIndex, rankIndex []int32, i int32) (int32, int32) {
	a := int32(0)
	l := int32(len(words))
	wordI := selectIndex[i>>5] >> 6
	for ; rankIndex[wordI+1] <= i; wordI++ {
	}
	w := words[wordI]
	ww := w
	base := wordI << 6
	findIth := int(i - rankIndex[wordI])
	offset := int32(0)
	ones := bits.OnesCount32(uint32(ww))
	if ones <= findIth {
		findIth -= ones
		offset |= 32
		ww >>= 32
	}
	ones = bits.OnesCount16(uint16(ww))
	if ones <= findIth {
		findIth -= ones
		offset |= 16
		ww >>= 16
	}
	ones = bits.OnesCount8(uint8(ww))
	if ones <= findIth {
		a = int32(select8Lookup[(ww>>5)&(0x7f8)|uint64(findIth-ones)]) + offset + 8
	} else {
		a = int32(select8Lookup[(ww&0xff)<<3|uint64(findIth)]) + offset
	}
	a += base
	w &= rMaskUpto[a&63]
	if w != 0 {
		return a, base + int32(bits.TrailingZeros64(w))
	}
	wordI++
	for ; wordI < l; wordI++ {
		w = words[wordI]
		if w != 0 {
			return a, wordI<<6 + int32(bits.TrailingZeros64(w))
		}
	}
	return a, l << 6
}

func indexSelect32R64(words []uint64) ([]int32, []int32) {
	l := len(words) << 6
	sidx := make([]int32, 0, len(words))

	ith := -1
	for i := 0; i < l; i++ {
		if words[i>>6]&(1<<uint(i&63)) != 0 {
			ith++
			if ith&31 == 0 {
				sidx = append(sidx, int32(i))
			}
		}
	}

	// clone to reduce cap to len
	sidx = append(sidx[:0:0], sidx...)
	return sidx, indexRank64(words, true)
}

func init() {
	initMasks()
	initSelectLookup()
}

var (
	mask      [65]uint64
	rMaskUpto [64]uint64
)

func initMasks() {
	for i := 0; i < 65; i++ {
		mask[i] = (1 << uint(i)) - 1
	}

	var maskUpto [64]uint64
	for i := 0; i < 64; i++ {
		maskUpto[i] = (1 << uint(i+1)) - 1
		rMaskUpto[i] = ^maskUpto[i]
	}
}

var select8Lookup [256 * 8]uint8

func initSelectLookup() {
	for i := 0; i < 256; i++ {
		w := uint8(i)
		for j := 0; j < 8; j++ {
			// x-th 1 in w
			// if x-th 1 is not found, it is 8
			x := bits.TrailingZeros8(w)
			w &= w - 1

			select8Lookup[i*8+j] = uint8(x)
		}
	}
}
