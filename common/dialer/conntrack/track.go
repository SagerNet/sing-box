package conntrack

import (
	"io"
	"sync"

	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/x/list"
)

var (
	connAccess     sync.RWMutex
	openConnection list.List[io.Closer]
)

func Count() int {
	return openConnection.Len()
}

func List() []io.Closer {
	connAccess.RLock()
	defer connAccess.RUnlock()
	connList := make([]io.Closer, 0, openConnection.Len())
	for element := openConnection.Front(); element != nil; element = element.Next() {
		connList = append(connList, element.Value)
	}
	return connList
}

func Close() {
	connAccess.Lock()
	defer connAccess.Unlock()
	for element := openConnection.Front(); element != nil; element = element.Next() {
		common.Close(element.Value)
		element.Value = nil
	}
	openConnection.Init()
}
