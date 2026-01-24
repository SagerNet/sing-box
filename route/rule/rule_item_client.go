package rule

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*ClientItem)(nil)

type ClientItem struct {
	clients   []string
	clientMap map[string]bool
}

func NewClientItem(clients []string) *ClientItem {
	clientMap := make(map[string]bool)
	for _, client := range clients {
		clientMap[client] = true
	}
	return &ClientItem{
		clients:   clients,
		clientMap: clientMap,
	}
}

func (r *ClientItem) Match(metadata *adapter.InboundContext) bool {
	return r.clientMap[metadata.Client]
}

func (r *ClientItem) String() string {
	if len(r.clients) == 1 {
		return F.ToString("client=", r.clients[0])
	}
	return F.ToString("client=[", strings.Join(r.clients, " "), "]")
}
