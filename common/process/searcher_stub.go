//go:build !linux && !windows && !darwin

package process

import (
	"os"
)

func NewSearcher(_ Config) (Searcher, error) {
	return nil, os.ErrInvalid
}
