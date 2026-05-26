package accesslog

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	M "github.com/sagernet/sing/common/metadata"
)

func TestServiceWriteRotateAndCleanup(t *testing.T) {
	baseTime := time.Date(2026, 5, 26, 10, 30, 0, 0, time.FixedZone("CST", 8*3600))
	tempDir := t.TempDir()
	oldFile := filepath.Join(tempDir, "access-2026-05-18-09.log")
	if err := os.WriteFile(oldFile, []byte("old\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := &Service{
		ctx:       context.Background(),
		logger:    log.NewNOPFactory().Logger(),
		path:      tempDir,
		retention: 7 * 24 * time.Hour,
	}
	now := baseTime
	service.now = func() time.Time {
		return now
	}
	if err := service.Start(adapter.StartStateInitialize); err != nil {
		t.Fatal(err)
	}
	service.write(adapter.InboundContext{
		User:   "alice",
		Domain: "example.com",
		Source: M.Socksaddr{Addr: M.ParseAddr("203.0.113.10")},
	})
	now = baseTime.Add(time.Hour)
	service.write(adapter.InboundContext{
		User:        "bob",
		Destination: M.Socksaddr{Fqdn: "example.org"},
		Source:      M.Socksaddr{Addr: M.ParseAddr("2001:db8::1")},
	})
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Fatalf("expected expired access log file removed, got err=%v", err)
	}
	firstEntry := decodeEntry(t, filepath.Join(tempDir, "access-2026-05-26-10.log"))
	if firstEntry.Name != "alice" || firstEntry.Domain != "example.com" || firstEntry.SourceIP != "203.0.113.10" {
		t.Fatalf("unexpected first entry: %+v", firstEntry)
	}
	secondEntry := decodeEntry(t, filepath.Join(tempDir, "access-2026-05-26-11.log"))
	if secondEntry.Name != "bob" || secondEntry.Domain != "example.org" || secondEntry.SourceIP != "2001:db8::1" {
		t.Fatalf("unexpected second entry: %+v", secondEntry)
	}
}

func decodeEntry(t *testing.T, path string) Entry {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var entry Entry
	if err := json.Unmarshal([]byte(strings.TrimSpace(string(content))), &entry); err != nil {
		t.Fatal(err)
	}
	return entry
}
