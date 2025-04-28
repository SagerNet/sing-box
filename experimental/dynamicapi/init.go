package dynamicapi

import (
	"github.com/sagernet/sing-box/experimental"
)

func init() {
	experimental.RegisterDynamicManagerConstructor(NewServer)
}
