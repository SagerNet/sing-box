//go:build !(windows || linux || darwin)

package settings

import (
	"context"
	"os"

	M "github.com/sagernet/sing/common/metadata"
)

func NewSystemProxy(ctx context.Context, serverAddr M.Socksaddr, supportSOCKS bool) (SystemProxy, error) {
	return nil, os.ErrInvalid
}
