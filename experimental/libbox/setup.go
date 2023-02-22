package libbox

import C "github.com/sagernet/sing-box/constant"

func SetBasePath(path string) {
	C.SetBasePath(path)
}

func Version() string {
	return C.Version
}
