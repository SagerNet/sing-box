package ssmapi

import (
	"sync"

	"github.com/sagernet/sing-box/adapter"
	E "github.com/sagernet/sing/common/exceptions"
)

type UserManager struct {
	access         sync.Mutex
	usersMap       map[string]string
	server         adapter.ManagedSSMServer
	trafficManager *TrafficManager
}

func NewUserManager(inbound adapter.ManagedSSMServer, trafficManager *TrafficManager) *UserManager {
	return &UserManager{
		usersMap:       make(map[string]string),
		server:         inbound,
		trafficManager: trafficManager,
	}
}

func (m *UserManager) postUpdate(updated bool) error {
	users := make([]string, 0, len(m.usersMap))
	uPSKs := make([]string, 0, len(m.usersMap))
	for username, password := range m.usersMap {
		users = append(users, username)
		uPSKs = append(uPSKs, password)
	}
	err := m.server.UpdateUsers(users, uPSKs)
	if err != nil {
		return err
	}
	if updated {
		m.trafficManager.UpdateUsers(users)
	}
	return nil
}

func (m *UserManager) List() []*UserObject {
	m.access.Lock()
	defer m.access.Unlock()

	users := make([]*UserObject, 0, len(m.usersMap))
	for username, password := range m.usersMap {
		users = append(users, &UserObject{
			UserName: username,
			Password: password,
		})
	}
	return users
}

func (m *UserManager) Add(username string, password string) error {
	m.access.Lock()
	defer m.access.Unlock()
	if _, found := m.usersMap[username]; found {
		return E.New("user ", username, " already exists")
	}
	m.usersMap[username] = password
	return m.postUpdate(true)
}

func (m *UserManager) Get(username string) (string, bool) {
	m.access.Lock()
	defer m.access.Unlock()
	if password, found := m.usersMap[username]; found {
		return password, true
	}
	return "", false
}

func (m *UserManager) Update(username string, password string) error {
	m.access.Lock()
	defer m.access.Unlock()
	m.usersMap[username] = password
	return m.postUpdate(true)
}

func (m *UserManager) Delete(username string) error {
	m.access.Lock()
	defer m.access.Unlock()
	delete(m.usersMap, username)
	return m.postUpdate(true)
}
