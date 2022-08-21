package debugio

import (
	"fmt"
	"reflect"

	"github.com/sagernet/sing/common"
)

func PrintUpstream(obj any) {
	for obj != nil {
		fmt.Println(reflect.TypeOf(obj))
		if u, ok := obj.(common.WithUpstream); !ok {
			break
		} else {
			obj = u.Upstream()
		}
	}
}
