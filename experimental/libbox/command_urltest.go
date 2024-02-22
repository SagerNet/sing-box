package libbox

import (
	"encoding/binary"
	"net"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/outbound"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/batch"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/rw"
	"github.com/sagernet/sing/service"
)

func (c *CommandClient) URLTest(groupTag string) error {
	conn, err := c.directConnect()
	if err != nil {
		return err
	}
	defer conn.Close()
	err = binary.Write(conn, binary.BigEndian, uint8(CommandURLTest))
	if err != nil {
		return err
	}
	err = rw.WriteVString(conn, groupTag)
	if err != nil {
		return err
	}
	return readError(conn)
}

func (s *CommandServer) handleURLTest(conn net.Conn) error {
	groupTag, err := rw.ReadVString(conn)
	if err != nil {
		return err
	}
	serviceNow := s.service
	if serviceNow == nil {
		return nil
	}
	abstractOutboundGroup, isLoaded := serviceNow.instance.Router().Outbound(groupTag)
	if !isLoaded {
		return writeError(conn, E.New("outbound group not found: ", groupTag))
	}
	outboundGroup, isOutboundGroup := abstractOutboundGroup.(adapter.OutboundGroup)
	if !isOutboundGroup {
		return writeError(conn, E.New("outbound is not a group: ", groupTag))
	}
	urlTest, isURLTest := abstractOutboundGroup.(*outbound.URLTest)
	if isURLTest {
		go urlTest.CheckOutbounds()
	} else {
		historyStorage := service.PtrFromContext[urltest.HistoryStorage](serviceNow.ctx)
		outbounds := common.Filter(common.Map(outboundGroup.All(), func(it string) adapter.Outbound {
			itOutbound, _ := serviceNow.instance.Router().Outbound(it)
			return itOutbound
		}), func(it adapter.Outbound) bool {
			if it == nil {
				return false
			}
			_, isGroup := it.(adapter.OutboundGroup)
			if isGroup {
				return false
			}
			return true
		})
		b, _ := batch.New(serviceNow.ctx, batch.WithConcurrencyNum[any](10))
		for _, detour := range outbounds {
			outboundToTest := detour
			outboundTag := outboundToTest.Tag()
			b.Go(outboundTag, func() (any, error) {
				t, err := urltest.URLTest(serviceNow.ctx, "", outboundToTest)
				if err != nil {
					historyStorage.DeleteURLTestHistory(outboundTag)
				} else {
					historyStorage.StoreURLTestHistory(outboundTag, &urltest.History{
						Time:  time.Now(),
						Delay: t,
					})
				}
				return nil, nil
			})
		}
	}
	return writeError(conn, nil)
}
