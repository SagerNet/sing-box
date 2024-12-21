//go:build !android

package libbox

type RawNetwork interface {
	stub()
}
