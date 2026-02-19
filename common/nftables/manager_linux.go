//go:build linux

package nftables

import (
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"github.com/google/nftables"
	"github.com/sagernet/sing/common/logger"
)

type linuxManager struct {
	conn   *nftables.Conn
	logger logger.ContextLogger
	sets   map[string](*nftables.Set)
	mutex  sync.Mutex
}

func newManager(options Options) (Manager, error) {
	return &linuxManager{
		conn:   nil,
		logger: options.Logger,
		sets:   make(map[string](*nftables.Set)),
	}, nil
}

func (m *linuxManager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	conn, err := nftables.New(nftables.AsLasting())

	if err != nil {
		return err
	}

	m.conn = conn

	return nil
}

func getNftableSet(c *nftables.Conn, name string) (*nftables.Set, error) {
	arr := strings.SplitN(name, ":", 3)
	if len(arr) != 3 {
		return nil, fmt.Errorf("Nftable-Set expr %s does not have 3 parts", name)
	}
	tableName, setName := arr[1], arr[2]
	class := nftables.TableFamilyINet
	switch arr[0] {
	case "inet":
		class = nftables.TableFamilyINet
	case "ip":
		class = nftables.TableFamilyIPv4
	case "ip6":
		class = nftables.TableFamilyIPv6
	default:
		return nil, fmt.Errorf("Nftables-Table family %s is unknown", arr[0])
	}
	table := &nftables.Table{
		Name:   tableName,
		Family: class,
	}
	set, err := c.GetSetByName(table, setName)
	if err != nil {
		return nil, err
	}
	return set, nil
}

func (m *linuxManager) AddAddress(setName string, address netip.Addr, ttl time.Duration, reason string) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	set := m.sets[setName]

	if set == nil {
		getSet, err := getNftableSet(m.conn, setName)
		if err != nil {
			return err
		}
		set = getSet
		m.sets[setName] = getSet
	}

	elem := nftables.SetElement{
		Key:     address.AsSlice(),
		Timeout: ttl,
		Comment: reason,
	}

	m.conn.SetAddElements(set, []nftables.SetElement{elem})
	return nil
}

func (m *linuxManager) Close() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.conn != nil {
		m.conn.CloseLasting()
		m.conn = nil
	}

	return nil
}

func (m *linuxManager) Flush() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.conn.Flush()
}
