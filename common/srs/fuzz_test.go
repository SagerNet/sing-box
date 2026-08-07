package srs_test

import (
	"bytes"
	"testing"

	"github.com/sagernet/sing-box/common/srs"
)

// FuzzSRSRead fuzzes the binary rule-set (.srs) reader with recover=false, matching how it is
// called on rule-set content in route/rule. It guards against panics and unbounded allocations
// when parsing untrusted rule-set data.
func FuzzSRSRead(f *testing.F) {
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = srs.Read(bytes.NewReader(data), false)
	})
}
