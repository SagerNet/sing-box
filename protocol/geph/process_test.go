//go:build !windows

package geph

import (
	"context"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGephProcessArgsAndFraming(t *testing.T) {
	dir := t.TempDir()
	helper := filepath.Join(dir, "geph5-helper.py")
	script := "#!/usr/bin/env python3\nimport sys, struct\nwhile True:\n    h = sys.stdin.buffer.read(2)\n    if len(h) != 2: break\n    n = struct.unpack('>H', h)[0]\n    p = sys.stdin.buffer.read(n)\n    if len(p) != n: break\n    sys.stdout.buffer.write(h + p)\n    sys.stdout.buffer.flush()\n"
	if err := os.WriteFile(helper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	p := newGephProcess(context.Background(), helper, "/tmp/geph5.yaml", []string{"--extra-test-arg"}, time.Second)
	if got := p.args(); len(got) != 4 || got[0] != "--config" || got[1] != "/tmp/geph5.yaml" || got[2] != "--stdio-vpn" || got[3] != "--extra-test-arg" {
		t.Fatalf("unexpected arguments: %#v", got)
	}
	if err := p.Start(); err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	packet := []byte{0x45, 0, 1, 2, 3}
	if err := p.sendPacket(packet); err != nil {
		t.Fatal(err)
	}
	select {
	case got := <-p.incoming:
		if string(got) != string(packet) {
			t.Fatalf("packet mismatch: %x", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for framed packet")
	}
}

func TestGephProcessRejectsOversizedPacket(t *testing.T) {
	p := newGephProcess(context.Background(), "/bin/false", "config", nil, time.Second)
	if err := p.sendPacket(make([]byte, 65536)); err == nil {
		t.Fatal("expected oversized packet error")
	}
}

func TestGephFramingIsBigEndian(t *testing.T) {
	var header [2]byte
	binary.BigEndian.PutUint16(header[:], 0x1234)
	if header != [2]byte{0x12, 0x34} {
		t.Fatalf("unexpected framing: %x", header)
	}
}
