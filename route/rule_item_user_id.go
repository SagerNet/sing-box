package route

import (
	"strings"

	"github.com/sagernet/sing-box/adapter"
	F "github.com/sagernet/sing/common/format"
)

var _ RuleItem = (*UserIdItem)(nil)

type UserIdItem struct {
	userIds   []int32
	userIdMap map[int32]bool
}

func NewUserIDItem(userIdList []int32) *UserIdItem {
	rule := &UserIdItem{
		userIds:   userIdList,
		userIdMap: make(map[int32]bool),
	}
	for _, userId := range userIdList {
		rule.userIdMap[userId] = true
	}
	return rule
}

func (r *UserIdItem) Match(metadata *adapter.InboundContext) bool {
	if metadata.ProcessInfo == nil || metadata.ProcessInfo.UserId == -1 {
		return false
	}
	return r.userIdMap[metadata.ProcessInfo.UserId]
}

func (r *UserIdItem) String() string {
	var description string
	pLen := len(r.userIds)
	if pLen == 1 {
		description = "user_id=" + F.ToString(r.userIds[0])
	} else {
		description = "user_id=[" + strings.Join(F.MapToString(r.userIds), " ") + "]"
	}
	return description
}
