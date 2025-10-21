package ccm

import (
	"sync"

	"github.com/sagernet/sing-box/option"
)

type UserManager struct {
	accessMutex sync.RWMutex
	tokenMap    map[string]string
}

func (m *UserManager) UpdateUsers(users []option.CCMUser) {
	m.accessMutex.Lock()
	defer m.accessMutex.Unlock()
	tokenMap := make(map[string]string, len(users))
	for _, user := range users {
		tokenMap[user.Token] = user.Name
	}
	m.tokenMap = tokenMap
}

func (m *UserManager) Authenticate(token string) (string, bool) {
	m.accessMutex.RLock()
	username, found := m.tokenMap[token]
	m.accessMutex.RUnlock()
	return username, found
}
