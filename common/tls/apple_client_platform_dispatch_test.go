//go:build darwin && cgo

package tls

import (
	"bytes"
	"strings"
	"testing"
)

func TestAppleTLSDispatchDataCopySegments(t *testing.T) {
	first := []byte("hello ")
	second := []byte("world")

	buffer := make([]byte, len(first)+len(second))
	n, errorMessage := appleTLSCopyDispatchDataForTest(first, second, buffer)
	if n < 0 {
		t.Fatalf("copy failed: %s", errorMessage)
	}
	if int(n) != len(buffer) {
		t.Fatalf("copied %d bytes, want %d", n, len(buffer))
	}
	if !bytes.Equal(buffer, []byte("hello world")) {
		t.Fatalf("unexpected copy result: %q", string(buffer))
	}
}

func TestAppleTLSDispatchDataCopyRejectsSmallBuffer(t *testing.T) {
	first := []byte("hello")
	second := []byte("world")

	buffer := make([]byte, len(first)+len(second)-1)
	n, errorMessage := appleTLSCopyDispatchDataForTest(first, second, buffer)
	if n != -1 {
		t.Fatalf("copied %d bytes, want error", n)
	}
	if !strings.Contains(errorMessage, "read buffer too small") {
		t.Fatalf("unexpected error: %q", errorMessage)
	}
}

func TestAppleTLSDispatchDataCopyEmpty(t *testing.T) {
	n, errorMessage := appleTLSCopyDispatchDataForTest(nil, nil, nil)
	if n != 0 {
		t.Fatalf("copied %d bytes, want 0", n)
	}
	if errorMessage != "" {
		t.Fatalf("unexpected error: %q", errorMessage)
	}
}
