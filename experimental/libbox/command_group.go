package libbox

import (
	"bufio"
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/protocol/group"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/varbin"
	"github.com/sagernet/sing/service"
)

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
	var interval int64
	err := binary.Read(conn, binary.BigEndian, &interval)
	if err != nil {
		return E.Cause(err, "read interval")
	}
	ticker := time.NewTicker(time.Duration(interval))
	defer ticker.Stop()
	ctx := connKeepAlive(conn)
	writer := bufio.NewWriter(conn)
	for {
		service := s.service
		if service != nil {
			err = writeGroups(writer, service)
			if err != nil {
				return err
			}
		} else {
			err = binary.Write(writer, binary.BigEndian, uint16(0))
			if err != nil {
				return err
			}
		}
		err = writer.Flush()
		if err != nil {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.urlTestUpdate:
		}
	}
}

type OutboundGroup struct {
	Tag        string
	Type       string
	Selectable bool
	Selected   string
	IsExpand   bool
	ItemList   []*OutboundGroupItem
}

func (g *OutboundGroup) GetItems() OutboundGroupItemIterator {
	return newIterator(g.ItemList)
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

func readGroups(reader io.Reader) (OutboundGroupIterator, error) {
	groups, err := varbin.ReadValue[[]*OutboundGroup](reader, binary.BigEndian)
	if err != nil {
		return nil, err
	}
	return newIterator(groups), nil
}

func writeGroups(writer io.Writer, boxService *BoxService) error {
	historyStorage := service.PtrFromContext[urltest.HistoryStorage](boxService.ctx)
	cacheFile := service.FromContext[adapter.CacheFile](boxService.ctx)
	outbounds := boxService.instance.Outbound().Outbounds()
	var iGroups []adapter.OutboundGroup
	for _, it := range outbounds {
		if group, isGroup := it.(adapter.OutboundGroup); isGroup {
			iGroups = append(iGroups, group)
		}
	}
	var groups []OutboundGroup
	for _, iGroup := range iGroups {
		var outboundGroup OutboundGroup
		outboundGroup.Tag = iGroup.Tag()
		outboundGroup.Type = iGroup.Type()
		_, outboundGroup.Selectable = iGroup.(*group.Selector)
		outboundGroup.Selected = iGroup.Now()
		if cacheFile != nil {
			if isExpand, loaded := cacheFile.LoadGroupExpand(outboundGroup.Tag); loaded {
				outboundGroup.IsExpand = isExpand
			}
		}

		for _, itemTag := range iGroup.All() {
			itemOutbound, isLoaded := boxService.instance.Outbound().Outbound(itemTag)
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
			outboundGroup.ItemList = append(outboundGroup.ItemList, &item)
		}
		if len(outboundGroup.ItemList) < 2 {
			continue
		}
		groups = append(groups, outboundGroup)
	}
	return varbin.Write(writer, binary.BigEndian, groups)
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
	err = varbin.Write(conn, binary.BigEndian, groupTag)
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
	groupTag, err := varbin.ReadValue[string](conn, binary.BigEndian)
	if err != nil {
		return err
	}
	var isExpand bool
	err = binary.Read(conn, binary.BigEndian, &isExpand)
	if err != nil {
		return err
	}
	serviceNow := s.service
	if serviceNow == nil {
		return writeError(conn, E.New("service not ready"))
	}
	cacheFile := service.FromContext[adapter.CacheFile](serviceNow.ctx)
	if cacheFile != nil {
		err = cacheFile.StoreGroupExpand(groupTag, isExpand)
		if err != nil {
			return writeError(conn, err)
		}
	}
	return writeError(conn, nil)
}
