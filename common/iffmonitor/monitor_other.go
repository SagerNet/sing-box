//go:build !linux

package iffmonitor

import (
	"os"

	"github.com/sagernet/sing-box/log"
)

func New(logger log.Logger) (InterfaceMonitor, error) {
	return nil, os.ErrInvalid
}
