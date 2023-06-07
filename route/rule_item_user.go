package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*UserItem)(nil)

type UserItem struct {
	users   []string
	userMap map[string]bool
}

func NewUserItem(users []string) *UserItem {
	userMap := make(map[string]bool)
	for _, protocol := range users {
		userMap[protocol] = true
	}
	return &UserItem{
		users:   users,
		userMap: userMap,
	}
}

func (r *UserItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || metadata.ProcessInfo.User == "" {
		return false
	}
	return r.userMap[metadata.ProcessInfo.User]
}

func (r *UserItem) String() string {
	if len(r.users) == 1 {
		return F.ToString("user=", r.users[0])
	}
	return F.ToString("user=[", strings.Join(r.users, " "), "]")
}
