//go:build !linux && !windows && !darwin

package process

import (
	"os"

	"github.com/sagernet/sing-box/log"
)

func NewSearcher(logger log.ContextLogger) (Searcher, error) {
	return nil, os.ErrInvalid
}
