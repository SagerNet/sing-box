package jstest

import (
	_ "embed"

	"github.com/sagernet/sing-box/script/modules/require"
)

//go:embed assert.js
var assertJS []byte

func NewRegistry() *require.Registry {
	return require.NewRegistry(require.WithFsEnable(true), require.WithLoader(func(path string) ([]byte, error) {
		switch path {
		case "assert.js":
			return assertJS, nil
		default:
			return require.DefaultSourceLoader(path)
		}
	}))
}
