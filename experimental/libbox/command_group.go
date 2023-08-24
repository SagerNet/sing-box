package libbox

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/outbound"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/service"
)

type OutboundGroup struct {
	Tag        string
	Type       string
	Selectable bool
	Selected   string
	IsExpand   bool
	items      []*OutboundGroupItem
}

func (g *OutboundGroup) GetItems() OutboundGroupItemIterator {
	return newIterator(g.items)
}

type OutboundGroupIterator interface {
	Next() *OutboundGroup
	HasNext() bool
}

type OutboundGroupItem struct {
	Tag          string
	Type         string
	URLTestTime  int64
	URLTestDelay int32
}

type OutboundGroupItemIterator interface {
	Next() *OutboundGroupItem
	HasNext() bool
}

func (c *CommandClient) handleGroupConn(conn net.Conn) {
	defer conn.Close()

	for {
		groups, err := readGroups(conn)
		if err != nil {
			c.handler.Disconnected(err.Error())
			return
		}
		c.handler.WriteGroups(groups)
	}
}

func (s *CommandServer) handleGroupConn(conn net.Conn) error {
	defer conn.Close()
	ctx := connKeepAlive(conn)
	for {
		service := s.service
		if service != nil {
			err := writeGroups(conn, service)
			if err != nil {
				return err
			}
		} else {
			err := binary.Write(conn, binary.BigEndian, uint16(0))
			if err != nil {
				return err
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.urlTestUpdate:
		}
	}
}

func readGroups(reader io.Reader) (OutboundGroupIterator, error) {
	var groupLength uint16
	err := binary.Read(reader, binary.BigEndian, &groupLength)
	if err != nil {
		return nil, err
	}

	groups := make([]*OutboundGroup, 0, groupLength)
	for i := 0; i < int(groupLength); i++ {
		var group OutboundGroup
		group.Tag, err = rw.ReadVString(reader)
		if err != nil {
			return nil, err
		}

		group.Type, err = rw.ReadVString(reader)
		if err != nil {
			return nil, err
		}

		err = binary.Read(reader, binary.BigEndian, &group.Selectable)
		if err != nil {
			return nil, err
		}

		group.Selected, err = rw.ReadVString(reader)
		if err != nil {
			return nil, err
		}

		err = binary.Read(reader, binary.BigEndian, &group.IsExpand)
		if err != nil {
			return nil, err
		}

		var itemLength uint16
		err = binary.Read(reader, binary.BigEndian, &itemLength)
		if err != nil {
			return nil, err
		}

		group.items = make([]*OutboundGroupItem, itemLength)
		for j := 0; j < int(itemLength); j++ {
			var item OutboundGroupItem
			item.Tag, err = rw.ReadVString(reader)
			if err != nil {
				return nil, err
			}

			item.Type, err = rw.ReadVString(reader)
			if err != nil {
				return nil, err
			}

			err = binary.Read(reader, binary.BigEndian, &item.URLTestTime)
			if err != nil {
				return nil, err
			}

			err = binary.Read(reader, binary.BigEndian, &item.URLTestDelay)
			if err != nil {
				return nil, err
			}

			group.items[j] = &item
		}
		groups = append(groups, &group)
	}
	return newIterator(groups), nil
}

func writeGroups(writer io.Writer, boxService *BoxService) error {
	historyStorage := service.PtrFromContext[urltest.HistoryStorage](boxService.ctx)
	var cacheFile adapter.ClashCacheFile
	if clashServer := boxService.instance.Router().ClashServer(); clashServer != nil {
		cacheFile = clashServer.CacheFile()
	}

	outbounds := boxService.instance.Router().Outbounds()
	var iGroups []adapter.OutboundGroup
	for _, it := range outbounds {
		if group, isGroup := it.(adapter.OutboundGroup); isGroup {
			iGroups = append(iGroups, group)
		}
	}
	var groups []OutboundGroup
	for _, iGroup := range iGroups {
		var group OutboundGroup
		group.Tag = iGroup.Tag()
		group.Type = iGroup.Type()
		_, group.Selectable = iGroup.(*outbound.Selector)
		group.Selected = iGroup.Now()
		if cacheFile != nil {
			if isExpand, loaded := cacheFile.LoadGroupExpand(group.Tag); loaded {
				group.IsExpand = isExpand
			}
		}

		for _, itemTag := range iGroup.All() {
			itemOutbound, isLoaded := boxService.instance.Router().Outbound(itemTag)
			if !isLoaded {
				continue
			}

			var item OutboundGroupItem
			item.Tag = itemTag
			item.Type = itemOutbound.Type()
			if history := historyStorage.LoadURLTestHistory(adapter.OutboundTag(itemOutbound)); history != nil {
				item.URLTestTime = history.Time.Unix()
				item.URLTestDelay = int32(history.Delay)
			}
			group.items = append(group.items, &item)
		}
		if len(group.items) < 2 {
			continue
		}
		groups = append(groups, group)
	}

	err := binary.Write(writer, binary.BigEndian, uint16(len(groups)))
	if err != nil {
		return err
	}
	for _, group := range groups {
		err = rw.WriteVString(writer, group.Tag)
		if err != nil {
			return err
		}
		err = rw.WriteVString(writer, group.Type)
		if err != nil {
			return err
		}
		err = binary.Write(writer, binary.BigEndian, group.Selectable)
		if err != nil {
			return err
		}
		err = rw.WriteVString(writer, group.Selected)
		if err != nil {
			return err
		}
		err = binary.Write(writer, binary.BigEndian, group.IsExpand)
		if err != nil {
			return err
		}
		err = binary.Write(writer, binary.BigEndian, uint16(len(group.items)))
		if err != nil {
			return err
		}
		for _, item := range group.items {
			err = rw.WriteVString(writer, item.Tag)
			if err != nil {
				return err
			}
			err = rw.WriteVString(writer, item.Type)
			if err != nil {
				return err
			}
			err = binary.Write(writer, binary.BigEndian, item.URLTestTime)
			if err != nil {
				return err
			}
			err = binary.Write(writer, binary.BigEndian, item.URLTestDelay)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *CommandClient) SetGroupExpand(groupTag string, isExpand bool) error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandGroupExpand))
	if err != nil {
		return err
	}
	err = rw.WriteVString(conn, groupTag)
	if err != nil {
		return err
	}
	err = binary.Write(conn, binary.BigEndian, isExpand)
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleSetGroupExpand(conn net.Conn) error {
	defer conn.Close()
	groupTag, err := rw.ReadVString(conn)
	if err != nil {
		return err
	}
	var isExpand bool
	err = binary.Read(conn, binary.BigEndian, &isExpand)
	if err != nil {
		return err
	}
	service := s.service
	if service == nil {
		return writeError(conn, E.New("service not ready"))
	}
	if clashServer := service.instance.Router().ClashServer(); clashServer != nil {
		if cacheFile := clashServer.CacheFile(); cacheFile != nil {
			err = cacheFile.StoreGroupExpand(groupTag, isExpand)
			if err != nil {
				return writeError(conn, err)
			}
		}
	}
	return writeError(conn, nil)
}
