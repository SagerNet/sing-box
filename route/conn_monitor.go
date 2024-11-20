package route

import (
	"context"
	"io"
	"reflect"
	"sync"
	"time"

	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/x/list"
)

type ConnectionMonitor struct {
	access      sync.RWMutex
	reloadChan  chan struct{}
	connections list.List[*monitorEntry]
}

type monitorEntry struct {
	ctx    context.Context
	closer io.Closer
}

func NewConnectionMonitor() *ConnectionMonitor {
	return &ConnectionMonitor{
		reloadChan: make(chan struct{}, 1),
	}
}

func (m *ConnectionMonitor) Add(ctx context.Context, closer io.Closer) N.CloseHandlerFunc {
	m.access.Lock()
	defer m.access.Unlock()
	element := m.connections.PushBack(&monitorEntry{
		ctx:    ctx,
		closer: closer,
	})
	select {
	case <-m.reloadChan:
		return nil
	default:
		select {
		case m.reloadChan <- struct{}{}:
		default:
		}
	}
	return func(it error) {
		m.access.Lock()
		defer m.access.Unlock()
		m.connections.Remove(element)
		select {
		case <-m.reloadChan:
		default:
			select {
			case m.reloadChan <- struct{}{}:
			default:
			}
		}
	}
}

func (m *ConnectionMonitor) Start() error {
	go m.monitor()
	return nil
}

func (m *ConnectionMonitor) Close() error {
	m.access.Lock()
	defer m.access.Unlock()
	close(m.reloadChan)
	for element := m.connections.Front(); element != nil; element = element.Next() {
		element.Value.closer.Close()
	}
	return nil
}

func (m *ConnectionMonitor) monitor() {
	var (
		selectCases []reflect.SelectCase
		elements    []*list.Element[*monitorEntry]
	)
	rootCase := reflect.SelectCase{
		Dir:  reflect.SelectRecv,
		Chan: reflect.ValueOf(m.reloadChan),
	}
	for {
		m.access.RLock()
		if m.connections.Len() == 0 {
			m.access.RUnlock()
			if _, loaded := <-m.reloadChan; !loaded {
				return
			} else {
				continue
			}
		}
		if len(elements) < m.connections.Len() {
			elements = make([]*list.Element[*monitorEntry], 0, m.connections.Len())
		}
		if len(selectCases) < m.connections.Len()+1 {
			selectCases = make([]reflect.SelectCase, 0, m.connections.Len()+1)
		}
		elements = elements[:0]
		selectCases = selectCases[:1]
		selectCases[0] = rootCase
		for element := m.connections.Front(); element != nil; element = element.Next() {
			elements = append(elements, element)
			selectCases = append(selectCases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(element.Value.ctx.Done()),
			})
		}
		m.access.RUnlock()
		selected, _, loaded := reflect.Select(selectCases)
		if selected == 0 {
			if !loaded {
				return
			} else {
				time.Sleep(time.Second)
				continue
			}
		}
		element := elements[selected-1]
		m.access.Lock()
		m.connections.Remove(element)
		m.access.Unlock()
		element.Value.closer.Close() // maybe go close
	}
}
