//go:build with_bgp

package bgp

import (
	"github.com/osrg/gobgp/v3/pkg/log"
	"github.com/sagernet/sing-box/common/json"
	singlog "github.com/sagernet/sing-box/log"
)

type bgplog struct {
	logger singlog.Logger
}

func (l *bgplog) Panic(msg string, fields log.Fields) {
	l.logger.Panic(msg, logJson(fields))
}

func (l *bgplog) Fatal(msg string, fields log.Fields) {
	l.logger.Fatal(msg, logJson(fields))
}

func (l *bgplog) Error(msg string, fields log.Fields) {
	l.logger.Error(msg, logJson(fields))
}

func (l *bgplog) Warn(msg string, fields log.Fields) {
	l.logger.Warn(msg, logJson(fields))
}

func (l *bgplog) Info(msg string, fields log.Fields) {
	l.logger.Info(msg, logJson(fields))
}

func (l *bgplog) Debug(msg string, fields log.Fields) {
}

func (l *bgplog) SetLevel(level log.LogLevel) {
}

func (l *bgplog) GetLevel() log.LogLevel {
	return 0
}

func logJson(fields log.Fields) string {
	mes, _ := json.Marshal(fields)
	return string(mes)
}
