package ccm

import (
	"sync"

	"github.com/sagernet/sing-box/option"
)

type UserManager struct {
	access   sync.RWMutex
	tokenMap map[string]string
}

func NewUserManager() *UserManager {
	return &UserManager{
		tokenMap: make(map[string]string),
	}
}

func (m *UserManager) UpdateUsers(users []option.CCMUser) {
	m.access.Lock()
	defer m.access.Unlock()
	tokenMap := make(map[string]string, len(users))
	for _, user := range users {
		tokenMap[user.Token] = user.Name
	}
	m.tokenMap = tokenMap
}

func (m *UserManager) Authenticate(token string) (string, bool) {
	m.access.RLock()
	username, found := m.tokenMap[token]
	m.access.RUnlock()
	return username, found
}
