package option

import (
	"context"
	"testing"

	"github.com/sagernet/sing/common/json"

	"github.com/stretchr/testify/require"
)

func TestUSBIPHexUint16UnmarshalJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		input       string
		expected    USBIPHexUint16
		errorSubstr string
	}{
		{name: "number", input: `7531`, expected: USBIPHexUint16(0x1d6b)},
		{name: "hex-with-prefix", input: `"0x1d6b"`, expected: USBIPHexUint16(0x1d6b)},
		{name: "hex-with-uppercase-prefix", input: `"0X1D6B"`, expected: USBIPHexUint16(0x1d6b)},
		{name: "hex-without-prefix", input: `"1d6b"`, expected: USBIPHexUint16(0x1d6b)},
		{name: "empty-string", input: `""`, expected: 0},
		{name: "out-of-range", input: `65536`, errorSubstr: "out of uint16 range"},
		{name: "invalid-hex", input: `"zzzz"`, errorSubstr: "parse usb id zzzz"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			var value USBIPHexUint16
			err := json.UnmarshalContext(context.Background(), []byte(testCase.input), &value)
			if testCase.errorSubstr != "" {
				require.ErrorContains(t, err, testCase.errorSubstr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, testCase.expected, value)
		})
	}
}

func TestUSBIPHexUint16MarshalJSON(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		input    USBIPHexUint16
		expected string
	}{
		{name: "zero", input: 0, expected: `""`},
		{name: "non-zero", input: USBIPHexUint16(0x1d6b), expected: `"0x1d6b"`},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			content, err := json.Marshal(testCase.input)
			require.NoError(t, err)
			require.Equal(t, testCase.expected, string(content))
		})
	}
}
