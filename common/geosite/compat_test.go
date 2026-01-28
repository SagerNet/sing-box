package geosite

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/sagernet/sing/common/varbin"

	"github.com/stretchr/testify/require"
)

// Old implementation using varbin reflection-based serialization

func oldWriteString(writer varbin.Writer, value string) error {
	//nolint:staticcheck
	return varbin.Write(writer, binary.BigEndian, value)
}

func oldWriteItem(writer varbin.Writer, item Item) error {
	//nolint:staticcheck
	return varbin.Write(writer, binary.BigEndian, item)
}

func oldReadString(reader varbin.Reader) (string, error) {
	//nolint:staticcheck
	return varbin.ReadValue[string](reader, binary.BigEndian)
}

func oldReadItem(reader varbin.Reader) (Item, error) {
	//nolint:staticcheck
	return varbin.ReadValue[Item](reader, binary.BigEndian)
}

func TestStringCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"single_char", "a"},
		{"ascii", "example.com"},
		{"utf8", "测试域名.中国"},
		{"special_chars", "\x00\xff\n\t"},
		{"127_bytes", strings.Repeat("x", 127)},
		{"128_bytes", strings.Repeat("x", 128)},
		{"16383_bytes", strings.Repeat("x", 16383)},
		{"16384_bytes", strings.Repeat("x", 16384)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Old write
			var oldBuf bytes.Buffer
			err := oldWriteString(&oldBuf, tc.input)
			require.NoError(t, err)

			// New write
			var newBuf bytes.Buffer
			err = writeString(&newBuf, tc.input)
			require.NoError(t, err)

			// Bytes must match
			require.Equal(t, oldBuf.Bytes(), newBuf.Bytes(),
				"mismatch for %q\nold: %x\nnew: %x", tc.name, oldBuf.Bytes(), newBuf.Bytes())

			// New write -> old read
			readBack, err := oldReadString(bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			require.Equal(t, tc.input, readBack)

			// Old write -> new read
			readBack2, err := readString(bufio.NewReader(bytes.NewReader(oldBuf.Bytes())))
			require.NoError(t, err)
			require.Equal(t, tc.input, readBack2)
		})
	}
}

func TestItemCompat(t *testing.T) {
	t.Parallel()

	// Note: varbin.Write has a bug where struct values (not pointers) don't write their fields
	// because field.CanSet() returns false for non-addressable values.
	// The old geosite code passed Item values to varbin.Write, which silently wrote nothing.
	// The new code correctly writes Type + Value using manual serialization.
	// This test verifies the new serialization format and round-trip correctness.

	cases := []struct {
		name  string
		input Item
	}{
		{"domain_empty", Item{Type: RuleTypeDomain, Value: ""}},
		{"domain_normal", Item{Type: RuleTypeDomain, Value: "example.com"}},
		{"domain_suffix", Item{Type: RuleTypeDomainSuffix, Value: ".example.com"}},
		{"domain_keyword", Item{Type: RuleTypeDomainKeyword, Value: "google"}},
		{"domain_regex", Item{Type: RuleTypeDomainRegex, Value: `^.*\.example\.com$`}},
		{"utf8_domain", Item{Type: RuleTypeDomain, Value: "测试.com"}},
		{"long_domain", Item{Type: RuleTypeDomainSuffix, Value: strings.Repeat("a", 200) + ".com"}},
		{"128_bytes_value", Item{Type: RuleTypeDomain, Value: strings.Repeat("x", 128)}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// New write
			var newBuf bytes.Buffer
			err := newBuf.WriteByte(byte(tc.input.Type))
			require.NoError(t, err)
			err = writeString(&newBuf, tc.input.Value)
			require.NoError(t, err)

			// Verify format: Type (1 byte) + Value (uvarint len + bytes)
			require.True(t, len(newBuf.Bytes()) >= 1, "output too short")
			require.Equal(t, byte(tc.input.Type), newBuf.Bytes()[0], "type byte mismatch")

			// New write -> old read (varbin can read correctly when given addressable target)
			readBack, err := oldReadItem(bufio.NewReader(bytes.NewReader(newBuf.Bytes())))
			require.NoError(t, err)
			require.Equal(t, tc.input, readBack)

			// New write -> new read
			reader := bufio.NewReader(bytes.NewReader(newBuf.Bytes()))
			typeByte, err := reader.ReadByte()
			require.NoError(t, err)
			value, err := readString(reader)
			require.NoError(t, err)
			require.Equal(t, tc.input, Item{Type: ItemType(typeByte), Value: value})
		})
	}
}

func TestGeositeWriteReadCompat(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		input map[string][]Item
	}{
		{
			"empty_map",
			map[string][]Item{},
		},
		{
			"single_code_empty_items",
			map[string][]Item{"test": {}},
		},
		{
			"single_code_single_item",
			map[string][]Item{"test": {{Type: RuleTypeDomain, Value: "a.com"}}},
		},
		{
			"single_code_multi_items",
			map[string][]Item{
				"test": {
					{Type: RuleTypeDomain, Value: "a.com"},
					{Type: RuleTypeDomainSuffix, Value: ".b.com"},
					{Type: RuleTypeDomainKeyword, Value: "keyword"},
					{Type: RuleTypeDomainRegex, Value: `^.*$`},
				},
			},
		},
		{
			"multi_code",
			map[string][]Item{
				"cn": {{Type: RuleTypeDomain, Value: "baidu.com"}, {Type: RuleTypeDomainSuffix, Value: ".cn"}},
				"us": {{Type: RuleTypeDomain, Value: "google.com"}},
				"jp": {{Type: RuleTypeDomainSuffix, Value: ".jp"}},
			},
		},
		{
			"utf8_values",
			map[string][]Item{
				"test": {
					{Type: RuleTypeDomain, Value: "测试.中国"},
					{Type: RuleTypeDomainSuffix, Value: ".テスト"},
				},
			},
		},
		{
			"large_items",
			generateLargeItems(1000),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// Write using new implementation
			var buf bytes.Buffer
			err := Write(&buf, tc.input)
			require.NoError(t, err)

			// Read back and verify
			reader, codes, err := NewReader(bytes.NewReader(buf.Bytes()))
			require.NoError(t, err)

			// Verify all codes exist
			codeSet := make(map[string]bool)
			for _, code := range codes {
				codeSet[code] = true
			}
			for code := range tc.input {
				require.True(t, codeSet[code], "missing code: %s", code)
			}

			// Verify items match
			for code, expectedItems := range tc.input {
				items, err := reader.Read(code)
				require.NoError(t, err)
				require.Equal(t, expectedItems, items, "items mismatch for code: %s", code)
			}
		})
	}
}

func generateLargeItems(count int) map[string][]Item {
	items := make([]Item, count)
	for i := 0; i < count; i++ {
		items[i] = Item{
			Type:  ItemType(i % 4),
			Value: strings.Repeat("x", i%200) + ".com",
		}
	}
	return map[string][]Item{"large": items}
}
