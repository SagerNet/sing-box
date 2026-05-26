package accesslog

import (
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service/filemanager"
)

const (
	defaultDirectory = "access"
	filePrefix       = "access-"
	fileSuffix       = ".log"
	fileTimeFormat   = "2006-01-02-15"
)

var _ adapter.LifecycleService = (*Service)(nil)
var _ adapter.ConnectionTracker = (*Service)(nil)

type Service struct {
	ctx       context.Context
	logger    log.ContextLogger
	path      string
	retention time.Duration
	now       func() time.Time

	access      sync.Mutex
	file        *os.File
	currentHour time.Time
}

type Entry struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	SourceIP string `json:"source_ip"`
	Time     string `json:"time"`
}

func New(ctx context.Context, logger log.ContextLogger, options *option.AccessLogOptions) *Service {
	if options == nil || !options.Enabled {
		return nil
	}
	path := options.Path
	if path == "" {
		path = defaultDirectory
	}
	return &Service{
		ctx:       ctx,
		logger:    logger,
		path:      filemanager.BasePath(ctx, path),
		retention: 7 * 24 * time.Hour,
		now:       time.Now,
	}
}

func (s *Service) Name() string {
	return "access-log"
}

func (s *Service) Start(stage adapter.StartStage) error {
	if stage != adapter.StartStateInitialize {
		return nil
	}
	return s.prepareLocked(s.now())
}

func (s *Service) Close() error {
	s.access.Lock()
	defer s.access.Unlock()
	if s.file == nil {
		return nil
	}
	err := s.file.Close()
	s.file = nil
	return err
}

func (s *Service) RoutedConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) net.Conn {
	s.write(metadata)
	return conn
}

func (s *Service) RoutedPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext, matchedRule adapter.Rule, matchOutbound adapter.Outbound) N.PacketConn {
	s.write(metadata)
	return conn
}

func (s *Service) write(metadata adapter.InboundContext) {
	now := s.now()
	content, err := json.Marshal(Entry{
		Name:     metadata.User,
		Domain:   accessDomain(metadata),
		SourceIP: accessSourceIP(metadata),
		Time:     now.Format(time.RFC3339Nano),
	})
	if err != nil {
		s.logger.Error("marshal access log entry: ", err)
		return
	}
	s.access.Lock()
	defer s.access.Unlock()
	err = s.prepareLocked(now)
	if err != nil {
		s.logger.Error("prepare access log file: ", err)
		return
	}
	_, err = s.file.Write(append(content, '\n'))
	if err != nil {
		s.logger.Error("write access log entry: ", err)
	}
}

func (s *Service) prepareLocked(now time.Time) error {
	if err := filemanager.MkdirAll(s.ctx, s.path, 0o755); err != nil {
		return err
	}
	hour := now.Truncate(time.Hour)
	if s.file != nil && hour.Equal(s.currentHour) {
		return nil
	}
	if s.file != nil {
		if err := s.file.Close(); err != nil {
			return err
		}
		s.file = nil
	}
	filePath := filepath.Join(s.path, filePrefix+hour.Format(fileTimeFormat)+fileSuffix)
	file, err := filemanager.OpenFile(s.ctx, filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	s.file = file
	s.currentHour = hour
	s.cleanupLocked(now)
	return nil
}

func (s *Service) cleanupLocked(now time.Time) {
	entries, err := os.ReadDir(s.path)
	if err != nil {
		s.logger.Error("read access log directory: ", err)
		return
	}
	cutoff := now.Add(-s.retention)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		hour, ok := parseFileHour(entry.Name(), now.Location())
		if !ok || !hour.Before(cutoff) {
			continue
		}
		err = os.Remove(filepath.Join(s.path, entry.Name()))
		if err != nil && !os.IsNotExist(err) {
			s.logger.Error("remove expired access log file: ", err)
		}
	}
}

func parseFileHour(name string, location *time.Location) (time.Time, bool) {
	if !strings.HasPrefix(name, filePrefix) || !strings.HasSuffix(name, fileSuffix) {
		return time.Time{}, false
	}
	rawHour := strings.TrimSuffix(strings.TrimPrefix(name, filePrefix), fileSuffix)
	hour, err := time.ParseInLocation(fileTimeFormat, rawHour, location)
	if err != nil {
		return time.Time{}, false
	}
	return hour, true
}

func accessDomain(metadata adapter.InboundContext) string {
	if metadata.Domain != "" {
		return metadata.Domain
	}
	return metadata.Destination.Fqdn
}

func accessSourceIP(metadata adapter.InboundContext) string {
	if metadata.Source.Addr.IsValid() {
		return metadata.Source.Addr.Unmap().String()
	}
	return ""
}
