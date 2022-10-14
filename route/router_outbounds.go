package route

import (
	"sync"

	"github.com/sagernet/sing-box/adapter"
)

// outboundsManager is the thread-safe outbound manager.
type outboundsManager struct {
	sync.RWMutex

	tags []string // tags keeps the order of outbounds
	all  map[string]adapter.Outbound
}

func newOutboundsManager() *outboundsManager {
	return &outboundsManager{
		all: make(map[string]adapter.Outbound),
	}
}

func (o *outboundsManager) Add(outbound adapter.Outbound) {
	o.Lock()
	defer o.Unlock()

	tag := outbound.Tag()
	if _, ok := o.all[tag]; ok {
		// update and return
		o.all[tag] = outbound
		return
	}

	o.all[tag] = outbound
	o.tags = append(o.tags, tag)
}

func (o *outboundsManager) Remove(tag string) {
	o.Lock()
	defer o.Unlock()

	if _, ok := o.all[tag]; !ok {
		return
	}
	delete(o.all, tag)
	o.tags = findDeleteElement(o.tags, tag)
}

func (o *outboundsManager) Get(tag string) (adapter.Outbound, bool) {
	o.RLock()
	defer o.RUnlock()

	outbound, ok := o.all[tag]
	return outbound, ok
}

func (o *outboundsManager) All() []adapter.Outbound {
	o.RLock()
	defer o.RUnlock()

	all := make([]adapter.Outbound, 0, len(o.tags))
	for _, tag := range o.tags {
		all = append(all, o.all[tag])
	}
	return all
}

func findDeleteElement[T comparable](slice []T, element T) []T {
	idx := -1
	for i := 0; i < len(slice); i++ {
		if slice[i] == element {
			idx = i
			break
		}
	}
	if idx < 0 {
		return slice
	}
	copy(slice[idx:], slice[idx+1:])
	return slice[:len(slice)-1]
}
