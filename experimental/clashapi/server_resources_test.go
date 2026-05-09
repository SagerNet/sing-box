package clashapi

import (
	"archive/zip"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadZIPZipSlip(t *testing.T) {
	tempDir := t.TempDir()

	var buf bytes.Buffer

	zw := zip.NewWriter(&buf)

	_, err := zw.Create("../../../pwned.txt")
	if err != nil {
		t.Fatal(err)
	}

	err = zw.Close()
	if err != nil {
		t.Fatal(err)
	}

	server := &Server{
		ctx: context.Background(),
	}

	err = server.downloadZIP(bytes.NewReader(buf.Bytes()), tempDir)

	if err == nil {
		t.Fatal("expected zip slip error")
	}

	outsidePath := filepath.Join(tempDir, "..", "..", "..", "pwned.txt")

	_, err = os.Stat(outsidePath)

	if !os.IsNotExist(err) {
		t.Fatal("zip slip created file outside target directory")
	}
}
