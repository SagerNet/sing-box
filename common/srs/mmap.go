package srs

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"os"
	"runtime"
	"unsafe"

	"github.com/sagernet/sing-box/common/ipset"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/domain"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
)

var mmapMagic = [4]byte{0x53, 0x52, 0x53, 0x4D} // SRSM

const (
	mmapFormatVersion = 1
	mmapHeaderSize    = 40
	mmapBlobEntrySize = 24
	mmapBlobSuccinct  = 1
	mmapBlobIPSet     = 2
)

type mmapStorage struct {
	release func()
}

type mmapBlob struct {
	kind    uint64
	matcher domain.Mmap
	set     *ipset.Set
}

type mmapWriter struct {
	blobs []mmapBlob
}

func (w *mmapWriter) writeMatcher(writer varbin.Writer, matcher *domain.Matcher) error {
	w.blobs = append(w.blobs, mmapBlob{kind: mmapBlobSuccinct, matcher: matcher.Mmap()})
	_, err := varbin.WriteUvarint(writer, uint64(len(w.blobs)-1))
	return err
}

func (w *mmapWriter) writeAdGuardMatcher(writer varbin.Writer, matcher *domain.AdGuardMatcher) error {
	w.blobs = append(w.blobs, mmapBlob{kind: mmapBlobSuccinct, matcher: matcher.Mmap()})
	_, err := varbin.WriteUvarint(writer, uint64(len(w.blobs)-1))
	return err
}

func (w *mmapWriter) writeIPSet(writer varbin.Writer, set *ipset.Set) error {
	w.blobs = append(w.blobs, mmapBlob{kind: mmapBlobIPSet, set: set})
	_, err := varbin.WriteUvarint(writer, uint64(len(w.blobs)-1))
	return err
}

func WriteMmap(file io.WriteSeeker, ruleSet option.PlainRuleSetCompat) error {
	var header [mmapHeaderSize]byte
	_, err := file.Write(header[:])
	if err != nil {
		return err
	}
	mmap := &mmapWriter{}
	writer := bufio.NewWriter(file)
	_, err = varbin.WriteUvarint(writer, uint64(len(ruleSet.Options.Rules)))
	if err != nil {
		return err
	}
	for _, rule := range ruleSet.Options.Rules {
		err = writeRule(writer, rule, ruleSet.Version, mmap)
		if err != nil {
			return err
		}
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	rulesEnd, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		return err
	}
	rulesLength := uint64(rulesEnd) - mmapHeaderSize
	blobTableOffset := alignMmapOffset(uint64(rulesEnd))
	_, err = writer.Write(make([]byte, blobTableOffset-uint64(rulesEnd)))
	if err != nil {
		return err
	}
	blobOffset := blobTableOffset + uint64(len(mmap.blobs))*mmapBlobEntrySize
	for _, blob := range mmap.blobs {
		length := blob.size()
		err = writeMmapUint64(writer, blob.kind, blobOffset, length)
		if err != nil {
			return err
		}
		blobOffset += length
	}
	for _, blob := range mmap.blobs {
		err = blob.write(writer)
		if err != nil {
			return err
		}
	}
	err = writer.Flush()
	if err != nil {
		return err
	}
	copy(header[0:4], mmapMagic[:])
	binary.NativeEndian.PutUint32(header[4:8], mmapFormatVersion)
	binary.NativeEndian.PutUint64(header[8:16], uint64(ruleSet.Version))
	binary.NativeEndian.PutUint64(header[16:24], rulesLength)
	binary.NativeEndian.PutUint64(header[24:32], blobTableOffset)
	binary.NativeEndian.PutUint64(header[32:40], uint64(len(mmap.blobs)))
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = file.Write(header[:])
	return err
}

func (b *mmapBlob) size() uint64 {
	switch b.kind {
	case mmapBlobSuccinct:
		return 40 +
			uint64(len(b.matcher.Leaves))*8 +
			uint64(len(b.matcher.LabelBitmap))*8 +
			alignMmapOffset(uint64(len(b.matcher.Labels))) +
			alignMmapOffset(uint64(len(b.matcher.Ranks))*4) +
			alignMmapOffset(uint64(len(b.matcher.Selects))*4)
	case mmapBlobIPSet:
		return 16 +
			uint64(len(b.set.Ranges4()))*8 +
			uint64(len(b.set.Ranges6()))*32
	default:
		panic("unknown mmap blob kind")
	}
}

func (b *mmapBlob) write(writer *bufio.Writer) error {
	switch b.kind {
	case mmapBlobSuccinct:
		err := writeMmapUint64(writer,
			uint64(len(b.matcher.Leaves)),
			uint64(len(b.matcher.LabelBitmap)),
			uint64(len(b.matcher.Labels)),
			uint64(len(b.matcher.Ranks)),
			uint64(len(b.matcher.Selects)),
		)
		if err != nil {
			return err
		}
		err = writeMmapArray(writer, b.matcher.Leaves)
		if err != nil {
			return err
		}
		err = writeMmapArray(writer, b.matcher.LabelBitmap)
		if err != nil {
			return err
		}
		err = writeMmapArray(writer, b.matcher.Labels)
		if err != nil {
			return err
		}
		err = writeMmapArray(writer, b.matcher.Ranks)
		if err != nil {
			return err
		}
		return writeMmapArray(writer, b.matcher.Selects)
	case mmapBlobIPSet:
		err := writeMmapUint64(writer, uint64(len(b.set.Ranges4())), uint64(len(b.set.Ranges6())))
		if err != nil {
			return err
		}
		err = writeMmapArray(writer, b.set.Ranges4())
		if err != nil {
			return err
		}
		return writeMmapArray(writer, b.set.Ranges6())
	default:
		panic("unknown mmap blob kind")
	}
}

func writeMmapUint64(writer *bufio.Writer, values ...uint64) error {
	var buffer [8]byte
	for _, value := range values {
		binary.NativeEndian.PutUint64(buffer[:], value)
		_, err := writer.Write(buffer[:])
		if err != nil {
			return err
		}
	}
	return nil
}

func writeMmapArray[T any](writer *bufio.Writer, values []T) error {
	if len(values) == 0 {
		return nil
	}
	length := uintptr(len(values)) * unsafe.Sizeof(values[0])
	_, err := writer.Write(unsafe.Slice((*byte)(unsafe.Pointer(&values[0])), length))
	if err != nil {
		return err
	}
	padding := alignMmapOffset(uint64(length)) - uint64(length)
	if padding > 0 {
		var zero [8]byte
		_, err = writer.Write(zero[:padding])
	}
	return err
}

func alignMmapOffset(offset uint64) uint64 {
	return (offset + 7) &^ 7
}

type mmapReader struct {
	data      []byte
	storage   *mmapStorage
	blobTable []byte
	blobCount uint64
}

func ReadMmap(file *os.File) (ruleSet option.PlainRuleSetCompat, err error) {
	stat, err := file.Stat()
	if err != nil {
		return
	}
	if stat.Size() < mmapHeaderSize {
		err = E.New("truncated rule-set mmap")
		return
	}
	data, release, err := mmapFile(file, int(stat.Size()))
	if err != nil {
		return
	}
	storage := &mmapStorage{release: release}
	cleanup := runtime.AddCleanup(storage, func(release func()) { release() }, release)
	defer func() {
		if err != nil {
			cleanup.Stop()
			release()
		}
	}()
	if [4]byte(data[0:4]) != mmapMagic {
		err = E.New("invalid rule-set mmap")
		return
	}
	if binary.NativeEndian.Uint32(data[4:8]) != mmapFormatVersion {
		err = E.New("unsupported rule-set mmap version")
		return
	}
	rulesLength := binary.NativeEndian.Uint64(data[16:24])
	blobTableOffset := binary.NativeEndian.Uint64(data[24:32])
	blobCount := binary.NativeEndian.Uint64(data[32:40])
	rules, err := mmapSection(data, mmapHeaderSize, rulesLength)
	if err != nil {
		return
	}
	blobTable, err := mmapSection(data, blobTableOffset, blobCount*mmapBlobEntrySize)
	if err != nil {
		return
	}
	mmap := &mmapReader{
		data:      data,
		storage:   storage,
		blobTable: blobTable,
		blobCount: blobCount,
	}
	reader := bytes.NewReader(rules)
	ruleSet.Version = uint8(binary.NativeEndian.Uint64(data[8:16]))
	length, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	for i := range length {
		var rule option.HeadlessRule
		rule, err = readRule(reader, false, 0, mmap)
		if err != nil {
			err = E.Cause(err, "read rule[", i, "]")
			return
		}
		ruleSet.Options.Rules = append(ruleSet.Options.Rules, rule)
	}
	return
}

func mmapSection(data []byte, offset uint64, length uint64) ([]byte, error) {
	if offset%8 != 0 || offset > uint64(len(data)) || length > uint64(len(data))-offset {
		return nil, E.New("rule-set mmap section out of bounds")
	}
	return data[offset : offset+length : offset+length], nil
}

func (r *mmapReader) blob(reader varbin.Reader, kind uint64) ([]byte, error) {
	index, err := binary.ReadUvarint(reader)
	if err != nil {
		return nil, err
	}
	if index >= r.blobCount {
		return nil, E.New("rule-set mmap blob index out of range")
	}
	entry := r.blobTable[index*mmapBlobEntrySize:]
	if binary.NativeEndian.Uint64(entry[0:8]) != kind {
		return nil, E.New("rule-set mmap blob kind mismatch")
	}
	return mmapSection(r.data, binary.NativeEndian.Uint64(entry[8:16]), binary.NativeEndian.Uint64(entry[16:24]))
}

func (r *mmapReader) readMatcher(reader varbin.Reader) (*domain.Matcher, error) {
	mmap, err := r.readSuccinctMmap(reader)
	if err != nil {
		return nil, err
	}
	return domain.NewMatcherFromMmap(mmap)
}

func (r *mmapReader) readAdGuardMatcher(reader varbin.Reader) (*domain.AdGuardMatcher, error) {
	mmap, err := r.readSuccinctMmap(reader)
	if err != nil {
		return nil, err
	}
	return domain.NewAdGuardMatcherFromMmap(mmap)
}

func (r *mmapReader) readSuccinctMmap(reader varbin.Reader) (mmap domain.Mmap, err error) {
	blob, err := r.blob(reader, mmapBlobSuccinct)
	if err != nil {
		return
	}
	if len(blob) < 40 {
		err = E.New("rule-set mmap blob truncated")
		return
	}
	var counts [5]uint64
	for i := range counts {
		counts[i] = binary.NativeEndian.Uint64(blob[i*8:])
	}
	cursor := blob[40:]
	mmap.Leaves, cursor, err = mmapArray[uint64](cursor, counts[0])
	if err != nil {
		return
	}
	mmap.LabelBitmap, cursor, err = mmapArray[uint64](cursor, counts[1])
	if err != nil {
		return
	}
	mmap.Labels, cursor, err = mmapArray[byte](cursor, counts[2])
	if err != nil {
		return
	}
	mmap.Ranks, cursor, err = mmapArray[int32](cursor, counts[3])
	if err != nil {
		return
	}
	mmap.Selects, _, err = mmapArray[int32](cursor, counts[4])
	if err != nil {
		return
	}
	mmap.Storage = r.storage
	return
}

func (r *mmapReader) readIPSet(reader varbin.Reader) (*ipset.Set, error) {
	blob, err := r.blob(reader, mmapBlobIPSet)
	if err != nil {
		return nil, err
	}
	if len(blob) < 16 {
		return nil, E.New("rule-set mmap blob truncated")
	}
	cursor := blob[16:]
	ranges4, cursor, err := mmapArray[ipset.Range4](cursor, binary.NativeEndian.Uint64(blob[0:8]))
	if err != nil {
		return nil, err
	}
	ranges6, _, err := mmapArray[ipset.Range6](cursor, binary.NativeEndian.Uint64(blob[8:16]))
	if err != nil {
		return nil, err
	}
	return ipset.FromRanges(ranges4, ranges6, r.storage)
}

func mmapArray[T any](data []byte, count uint64) ([]T, []byte, error) {
	var zero T
	size := uint64(unsafe.Sizeof(zero))
	if count > uint64(len(data))/size {
		return nil, nil, E.New("rule-set mmap array out of bounds")
	}
	length := alignMmapOffset(count * size)
	if length > uint64(len(data)) {
		return nil, nil, E.New("rule-set mmap array out of bounds")
	}
	if count == 0 {
		return nil, data[length:], nil
	}
	return unsafe.Slice((*T)(unsafe.Pointer(&data[0])), count), data[length:], nil
}
