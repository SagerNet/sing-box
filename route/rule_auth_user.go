package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*AuthUserItem)(nil)

type AuthUserItem struct {
	users   []string
	userMap map[string]bool
}

func NewAuthUserItem(users []string) *AuthUserItem {
	userMap := make(map[string]bool)
	for _, protocol := range users {
		userMap[protocol] = true
	}
	return &AuthUserItem{
		users:   users,
		userMap: userMap,
	}
}

func (r *AuthUserItem) Match(metadata *adapter.InboundContext) bool {
	return r.userMap[metadata.User]
}

func (r *AuthUserItem) String() string {
	if len(r.users) == 1 {
		return F.ToString("auth_user=", r.users[0])
	}
	return F.ToString("auth_user=[", strings.Join(r.users, " "), "]")
}
