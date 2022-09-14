package tools

import (
	"bytes"
	"crypto/rand"
	"io"
)

func AppendRandBytes(b *bytes.Buffer, length int) {
	b.ReadFrom(io.LimitReader(rand.Reader, int64(length)))
}
