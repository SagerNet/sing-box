package srs

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"testing"

	C "github.com/sagernet/sing-box/constant"

	"github.com/stretchr/testify/require"
)

func craftRuleSet(body []byte) []byte {
	var buffer bytes.Buffer
	buffer.Write(MagicBytes[:])
	buffer.WriteByte(C.RuleSetVersionCurrent)
	compressWriter := zlib.NewWriter(&buffer)
	compressWriter.Write(body)
	compressWriter.Close()
	return buffer.Bytes()
}

func uvarint(value uint64) []byte {
	return binary.AppendUvarint(nil, value)
}

func TestReadMalformed(t *testing.T) {
	t.Parallel()
	defaultRule := []byte{0x00}
	cases := []struct {
		name string
		body []byte
	}{
		{"huge_rule_count", uvarint(1 << 40)},
		{"huge_string_count", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemDomainKeyword}, uvarint(1 << 40)}, nil)},
		{"huge_string_length", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemDomainKeyword}, uvarint(1), uvarint(1 << 40)}, nil)},
		{"huge_uint16_count", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemQueryType}, uvarint(1 << 40)}, nil)},
		{"huge_ip_range_count", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemIPCIDR, 0x01}, binary.BigEndian.AppendUint64(nil, 1<<40)}, nil)},
		{"bad_ip_range_address_length", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemIPCIDR, 0x01}, binary.BigEndian.AppendUint64(nil, 1), uvarint(99)}, nil)},
		{"huge_prefix_address_length", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemDefaultInterfaceAddress}, uvarint(1), uvarint(1 << 40)}, nil)},
		{"empty_domain_matcher", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemDomain, 0x00}, uvarint(0), uvarint(0), uvarint(0)}, nil)},
		{"huge_domain_matcher_bitmap", bytes.Join([][]byte{uvarint(1), defaultRule, {ruleItemDomain, 0x00}, uvarint(1 << 40)}, nil)},
		{"deep_logical_nesting", bytes.Join([][]byte{uvarint(1), bytes.Repeat([]byte{0x01, 0x00, 0x01}, 10000)}, nil)},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			_, err := Read(bytes.NewReader(craftRuleSet(testCase.body)), false)
			require.Error(t, err)
		})
	}
}
