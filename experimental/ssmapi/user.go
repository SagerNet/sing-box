package ssmapi

import (
	"sync"

	E "github.com/sagernet/sing/common/exceptions"
)

type UserManager struct {
	access         sync.Mutex
	usersMap       map[string]string
	nodes          []Node
	trafficManager *TrafficManager
}

func NewUserManager(nodes []Node, trafficManager *TrafficManager) *UserManager {
	return &UserManager{
		usersMap:       make(map[string]string),
		nodes:          nodes,
		trafficManager: trafficManager,
	}
}

func (m *UserManager) postUpdate() error {
	users := make([]string, 0, len(m.usersMap))
	uPSKs := make([]string, 0, len(m.usersMap))
	for username, password := range m.usersMap {
		users = append(users, username)
		uPSKs = append(uPSKs, password)
	}
	for _, node := range m.nodes {
		err := node.UpdateUsers(users, uPSKs)
		if err != nil {
			return err
		}
	}
	m.trafficManager.UpdateUsers(users)
	return nil
}

func (m *UserManager) List() []*SSMUserObject {
	m.access.Lock()
	defer m.access.Unlock()

	users := make([]*SSMUserObject, 0, len(m.usersMap))
	for username, password := range m.usersMap {
		users = append(users, &SSMUserObject{
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
		return E.New("user", username, "already exists")
	}
	m.usersMap[username] = password
	return m.postUpdate()
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
	return m.postUpdate()
}

func (m *UserManager) Delete(username string) error {
	m.access.Lock()
	defer m.access.Unlock()
	delete(m.usersMap, username)
	return m.postUpdate()
}
