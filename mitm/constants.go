package mitm

import (
	"encoding/base64"

	"github.com/sagernet/sing/common"
)

var surgeTinyGif = common.OnceValue(func() []byte {
	return common.Must1(base64.StdEncoding.DecodeString("R0lGODlhAQABAAAAACH5BAEAAAAALAAAAAABAAEAAAIBAAA="))
})
