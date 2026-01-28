package srs

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"net/netip"
	"strings"
	"testing"
	"unsafe"

	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/varbin"

	"github.com/stretchr/testify/require"
	"go4.org/netipx"
)

// Old implementations using varbin reflection-based serialization

func oldWriteStringSlice(writer varbin.Writer, value []string) error {
	//nolint:staticcheck
	return varbin.Write(writer, binary.BigEndian, value)
}

func oldReadStringSlice(reader varbin.Reader) ([]string, error) {
	//nolint:staticcheck
	return varbin.ReadValue[[]string](reader, binary.BigEndian)
}

func oldWriteUint8Slice[E ~uint8](writer varbin.Writer, value []E) error {
	//nolint:staticcheck
	return varbin.Write(writer, binary.BigEndian, value)
}

func oldReadUint8Slice[E ~uint8](reader varbin.Reader) ([]E, error) {
	//nolint:staticcheck
	return varbin.ReadValue[[]E](reader, binary.BigEndian)
}

func oldWriteUint16Slice(writer varbin.Writer, value []uint16) error {
	//nolint:staticcheck
	return varbin.Write(writer, binary.BigEndian, value)
}

func oldReadUint16Slice(reader varbin.Reader) ([]uint16, error) {
	//nolint:staticcheck
	return varbin.ReadValue[[]uint16](reader, binary.BigEndian)
}

func oldWritePrefix(writer varbin.Writer, prefix netip.Prefix) error {
	//nolint:staticcheck
	err := varbin.Write(writer, binary.BigEndian, prefix.Addr().AsSlice())
	if err != nil {
		return err
	}
	return binary.Write(writer, binary.BigEndian, uint8(prefix.Bits()))
}

type oldIPRangeData struct {
	From []byte
	To   []byte
}

// Note: The old writeIPSet had a bug where varbin.Write(writer, binary.BigEndian, data)
// with a struct VALUE (not pointer) silently wrote nothing because field.CanSet() returned false.
// This caused IP range data to be missing from the output.
// The new implementation correctly writes all range data.
//
// The old readIPSet used varbin.Read with a pre-allocated slice, which worked because
// slice elements are addressable and CanSet() returns true for them.
//
// For compatibility testing, we verify:
// 1. New write produces correct output with range data
// 2. New read can parse the new format correctly
// 3. Round-trip works correctly

func oldReadIPSet(reader varbin.Reader) (*netipx.IPSet, error) {
	version, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	if version != 1 {
		return nil, err
	}
	var length uint64
	err = binary.Read(reader, binary.BigEndian, &length)
	if err != nil {
		return nil, err
	}
	ranges := make([]oldIPRangeData, length)
	//nolint:staticcheck
	err = varbin.Read(reader, binary.BigEndian, &ranges)
	if err != nil {
		return nil, err
	}
	mySet := &myIPSet{
		rr: make([]myIPRange, len(ranges)),
	}
	for i, rangeData := range ranges {
		mySet.rr[i].from = M.AddrFromIP(rangeData.From)
		mySet.rr[i].to = M.AddrFromIP(rangeData.To)
	}
	return (*netipx.IPSet)(unsafe.Pointer(mySet)), nil
}

// New write functions (without itemType prefix for testing)

func newWriteStringSlice(writer varbin.Writer, value []string) error {
	_, err := varbin.WriteUvarint(writer, uint64(len(value)))
	if err != nil {
		return err
	}
	for _, s := range value {
		_, err = varbin.WriteUvarint(writer, uint64(len(s)))
		if err != nil {
			return err
		}
		_, err = writer.Write([]byte(s))
		if err != nil {
			return err
		}
	}
	return nil
}

func newWriteUint8Slice[E ~uint8](writer varbin.Writer, value []E) error {
	_, err := varbin.WriteUvarint(writer, uint64(len(value)))
	if err != nil {
		return err
	}
	_, err = writer.Write(*(*[]byte)(unsafe.Pointer(&value)))
	return err
}

func newWriteUint16Slice(writer varbin.Writer, value []uint16) error {
	_, err := varbin.WriteUvarint(writer, uint64(len(value)))
	if err != nil {
		return err
	}
	return binary.Write(writer, binary.BigEndian, value)
}

func newWritePrefix(writer varbin.Writer, prefix netip.Prefix) error {
	addrSlice := prefix.Addr().AsSlice()
	_, err := varbin.WriteUvarint(writer, uint64(len(addrSlice)))
	if err != nil {
		return err
	}
	_, err = writer.Write(addrSlice)
	if err != nil {
		return err
	}
	return writer.WriteByte(uint8(prefix.Bits()))
}

// Tests

func TestStringSliceCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input []string
	}{
		{"nil", nil},
		{"empty", []string{}},
		{"single_empty", []string{""}},
		{"single", []string{"test"}},
		{"multi", []string{"a", "b", "c"}},
		{"with_empty", []string{"a", "", "c"}},
		{"utf8", []string{"测试", "テスト", "тест"}},
		{"long_string", []string{strings.Repeat("x", 128)}},
		{"many_elements", generateStrings(128)},
		{"many_elements_256", generateStrings(256)},
		{"127_byte_string", []string{strings.Repeat("x", 127)}},
		{"128_byte_string", []string{strings.Repeat("x", 128)}},
		{"mixed_lengths", []string{"a", strings.Repeat("b", 100), "", strings.Repeat("c", 200)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Old write
			var oldBuf bytes.Buffer
			err := oldWriteStringSlice(&oldBuf, tc.input)
			require.NoError(t, err)

			// New write
			var newBuf bytes.Buffer
			err = newWriteStringSlice(&newBuf, tc.input)
			require.NoError(t, err)

			// Bytes must match
			require.Equal(t, oldBuf.Bytes(), newBuf.Bytes(),
				"mismatch for %q\nold: %x\nnew: %x", tc.name, oldBuf.Bytes(), newBuf.Bytes())

			// New write -> old read
			readBack, err := oldReadStringSlice(bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			requireStringSliceEqual(t, tc.input, readBack)

			// Old write -> new read
			readBack2, err := readRuleItemString(bufio.NewReader(bytes.NewReader(oldBuf.Bytes())))
			require.NoError(t, err)
			requireStringSliceEqual(t, tc.input, readBack2)
		})
	}
}

func TestUint8SliceCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input []uint8
	}{
		{"nil", nil},
		{"empty", []uint8{}},
		{"single_zero", []uint8{0}},
		{"single_max", []uint8{255}},
		{"multi", []uint8{0, 1, 127, 128, 255}},
		{"boundary", []uint8{0x00, 0x7f, 0x80, 0xff}},
		{"sequential", generateUint8Slice(256)},
		{"127_elements", generateUint8Slice(127)},
		{"128_elements", generateUint8Slice(128)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Old write
			var oldBuf bytes.Buffer
			err := oldWriteUint8Slice(&oldBuf, tc.input)
			require.NoError(t, err)

			// New write
			var newBuf bytes.Buffer
			err = newWriteUint8Slice(&newBuf, tc.input)
			require.NoError(t, err)

			// Bytes must match
			require.Equal(t, oldBuf.Bytes(), newBuf.Bytes(),
				"mismatch for %q\nold: %x\nnew: %x", tc.name, oldBuf.Bytes(), newBuf.Bytes())

			// New write -> old read
			readBack, err := oldReadUint8Slice[uint8](bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			requireUint8SliceEqual(t, tc.input, readBack)

			// Old write -> new read
			readBack2, err := readRuleItemUint8[uint8](bufio.NewReader(bytes.NewReader(oldBuf.Bytes())))
			require.NoError(t, err)
			requireUint8SliceEqual(t, tc.input, readBack2)
		})
	}
}

func TestUint16SliceCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input []uint16
	}{
		{"nil", nil},
		{"empty", []uint16{}},
		{"single_zero", []uint16{0}},
		{"single_max", []uint16{65535}},
		{"multi", []uint16{0, 255, 256, 32767, 32768, 65535}},
		{"ports", []uint16{80, 443, 8080, 8443}},
		{"127_elements", generateUint16Slice(127)},
		{"128_elements", generateUint16Slice(128)},
		{"256_elements", generateUint16Slice(256)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Old write
			var oldBuf bytes.Buffer
			err := oldWriteUint16Slice(&oldBuf, tc.input)
			require.NoError(t, err)

			// New write
			var newBuf bytes.Buffer
			err = newWriteUint16Slice(&newBuf, tc.input)
			require.NoError(t, err)

			// Bytes must match
			require.Equal(t, oldBuf.Bytes(), newBuf.Bytes(),
				"mismatch for %q\nold: %x\nnew: %x", tc.name, oldBuf.Bytes(), newBuf.Bytes())

			// New write -> old read
			readBack, err := oldReadUint16Slice(bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			requireUint16SliceEqual(t, tc.input, readBack)

			// Old write -> new read
			readBack2, err := readRuleItemUint16(bufio.NewReader(bytes.NewReader(oldBuf.Bytes())))
			require.NoError(t, err)
			requireUint16SliceEqual(t, tc.input, readBack2)
		})
	}
}

func TestPrefixCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input netip.Prefix
	}{
		{"ipv4_0", netip.MustParsePrefix("0.0.0.0/0")},
		{"ipv4_8", netip.MustParsePrefix("10.0.0.0/8")},
		{"ipv4_16", netip.MustParsePrefix("192.168.0.0/16")},
		{"ipv4_24", netip.MustParsePrefix("192.168.1.0/24")},
		{"ipv4_32", netip.MustParsePrefix("1.2.3.4/32")},
		{"ipv6_0", netip.MustParsePrefix("::/0")},
		{"ipv6_64", netip.MustParsePrefix("2001:db8::/64")},
		{"ipv6_128", netip.MustParsePrefix("::1/128")},
		{"ipv6_full", netip.MustParsePrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7334/128")},
		{"ipv4_private", netip.MustParsePrefix("172.16.0.0/12")},
		{"ipv6_link_local", netip.MustParsePrefix("fe80::/10")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Old write
			var oldBuf bytes.Buffer
			err := oldWritePrefix(&oldBuf, tc.input)
			require.NoError(t, err)

			// New write
			var newBuf bytes.Buffer
			err = newWritePrefix(&newBuf, tc.input)
			require.NoError(t, err)

			// Bytes must match
			require.Equal(t, oldBuf.Bytes(), newBuf.Bytes(),
				"mismatch for %q\nold: %x\nnew: %x", tc.name, oldBuf.Bytes(), newBuf.Bytes())

			// New write -> new read (no old read for prefix)
			readBack, err := readPrefix(bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			require.Equal(t, tc.input, readBack)

			// Old write -> new read
			readBack2, err := readPrefix(bufio.NewReader(bytes.NewReader(oldBuf.Bytes())))
			require.NoError(t, err)
			require.Equal(t, tc.input, readBack2)
		})
	}
}

func TestIPSetCompat(t *testing.T) {
	t.Parallel()

	// Note: The old writeIPSet was buggy (varbin.Write with struct values wrote nothing).
	// This test verifies the new implementation writes correct data and round-trips correctly.

	cases := []struct {
		name  string
		input *netipx.IPSet
	}{
		{"single_ipv4", buildIPSet("1.2.3.4")},
		{"ipv4_range", buildIPSet("192.168.0.0/16")},
		{"multi_ipv4", buildIPSet("10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16")},
		{"single_ipv6", buildIPSet("::1")},
		{"ipv6_range", buildIPSet("2001:db8::/32")},
		{"mixed", buildIPSet("10.0.0.0/8", "::1", "2001:db8::/32")},
		{"large", buildLargeIPSet(100)},
		{"adjacent_ranges", buildIPSet("192.168.0.0/24", "192.168.1.0/24", "192.168.2.0/24")},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// New write
			var newBuf bytes.Buffer
			err := writeIPSet(&newBuf, tc.input)
			require.NoError(t, err)

			// Verify format starts with version byte (1) + uint64 count
			require.True(t, len(newBuf.Bytes()) >= 9, "output too short")
			require.Equal(t, byte(1), newBuf.Bytes()[0], "version byte mismatch")

			// New write -> old read (varbin.Read with pre-allocated slice works correctly)
			readBack, err := oldReadIPSet(bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			requireIPSetEqual(t, tc.input, readBack)

			// New write -> new read
			readBack2, err := readIPSet(bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			requireIPSetEqual(t, tc.input, readBack2)
		})
	}
}

// Helper functions

func generateStrings(count int) []string {
	result := make([]string, count)
	for i := range result {
		result[i] = strings.Repeat("x", i%50)
	}
	return result
}

func generateUint8Slice(count int) []uint8 {
	result := make([]uint8, count)
	for i := range result {
		result[i] = uint8(i % 256)
	}
	return result
}

func generateUint16Slice(count int) []uint16 {
	result := make([]uint16, count)
	for i := range result {
		result[i] = uint16(i * 257)
	}
	return result
}

func buildIPSet(cidrs ...string) *netipx.IPSet {
	var builder netipx.IPSetBuilder
	for _, cidr := range cidrs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			addr, err := netip.ParseAddr(cidr)
			if err != nil {
				panic(err)
			}
			builder.Add(addr)
		} else {
			builder.AddPrefix(prefix)
		}
	}
	set, _ := builder.IPSet()
	return set
}

func buildLargeIPSet(count int) *netipx.IPSet {
	var builder netipx.IPSetBuilder
	for i := 0; i < count; i++ {
		prefix := netip.PrefixFrom(netip.AddrFrom4([4]byte{10, byte(i / 256), byte(i % 256), 0}), 24)
		builder.AddPrefix(prefix)
	}
	set, _ := builder.IPSet()
	return set
}

func requireStringSliceEqual(t *testing.T, expected, actual []string) {
	t.Helper()
	if len(expected) == 0 && len(actual) == 0 {
		return
	}
	require.Equal(t, expected, actual)
}

func requireUint8SliceEqual(t *testing.T, expected, actual []uint8) {
	t.Helper()
	if len(expected) == 0 && len(actual) == 0 {
		return
	}
	require.Equal(t, expected, actual)
}

func requireUint16SliceEqual(t *testing.T, expected, actual []uint16) {
	t.Helper()
	if len(expected) == 0 && len(actual) == 0 {
		return
	}
	require.Equal(t, expected, actual)
}

func requireIPSetEqual(t *testing.T, expected, actual *netipx.IPSet) {
	t.Helper()
	expectedRanges := expected.Ranges()
	actualRanges := actual.Ranges()
	require.Equal(t, len(expectedRanges), len(actualRanges), "range count mismatch")
	for i := range expectedRanges {
		require.Equal(t, expectedRanges[i].From(), actualRanges[i].From(), "range[%d].from mismatch", i)
		require.Equal(t, expectedRanges[i].To(), actualRanges[i].To(), "range[%d].to mismatch", i)
	}
}
