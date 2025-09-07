package wsc

import (
	"errors"
	"io"
	"math"
	"net"
	"sync"
	"sync/atomic"

	"goftp.io/server/v2/ratelimit"
)

const connReadSize = 2048

type connData struct {
	time   int64
	id     int
	reader io.Reader
	writer io.Writer
}

type wscUser struct {
	mu                    sync.Mutex
	id                    int64
	usedTrafficBytes      atomic.Int64
	reportedTrafficBytes  atomic.Int64
	lastTrafficUpdateTick atomic.Int64
	conns                 map[net.Conn]connData
	rateLimit             int64
	maxConnCount          int
	usedIds               []bool
}

func (manager *wscUserManager) newUser(id int64, usedTrafficBytes int64, maxConnCount int, rateLimit int64) *wscUser {
	user := &wscUser{
		id:           id,
		conns:        make(map[net.Conn]connData, maxConnCount),
		rateLimit:    rateLimit,
		usedIds:      make([]bool, maxConnCount),
		maxConnCount: maxConnCount,
	}
	user.usedTrafficBytes.Store(usedTrafficBytes)
	user.reportedTrafficBytes.Store(0)
	user.lastTrafficUpdateTick.Store(0)
	return user
}

func (user *wscUser) connReader(conn net.Conn) (io.Reader, error) {
	user.mu.Lock()
	defer user.mu.Unlock()
	if d, found := user.conns[conn]; found {
		return d.reader, nil
	}
	return nil, errors.New("connection doesn't exist")
}

func (user *wscUser) connWriter(conn net.Conn) (io.Writer, error) {
	user.mu.Lock()
	defer user.mu.Unlock()
	if d, found := user.conns[conn]; found {
		return d.writer, nil
	}
	return nil, errors.New("connection doesn't exist")
}

func (user *wscUser) connCount() int {
	user.mu.Lock()
	defer user.mu.Unlock()
	return len(user.conns)
}

func (user *wscUser) addConn(conn net.Conn) (net.Conn, error) {
	user.mu.Lock()
	defer user.mu.Unlock()

	if _, exists := user.conns[conn]; exists {
		return nil, errors.New("connection already exists")
	}

	var selectedConn net.Conn = nil
	selectedConnId := 0
	if user.maxConnCount > 0 {
		if len(user.conns) >= user.maxConnCount {
			minTime := int64(math.MaxInt64)
			for c, d := range user.conns {
				if d.time < minTime {
					minTime = d.time
					selectedConn = c
					selectedConnId = d.id
				}
			}
			if selectedConn != nil {
				delete(user.conns, selectedConn)
			}
		} else {
			for i := 0; i < user.maxConnCount; i++ {
				if !user.usedIds[i] {
					selectedConnId = i
					user.usedIds[i] = true
					break
				}
			}
		}
	}

	user.conns[conn] = connData{
		time:   nowns(),
		id:     selectedConnId,
		reader: ratelimit.Reader(conn, ratelimit.New(user.rateLimit)),
		writer: ratelimit.Writer(conn, ratelimit.New(user.rateLimit)),
	}

	return selectedConn, nil
}

func (user *wscUser) removeConn(conn net.Conn) error {
	user.mu.Lock()
	defer user.mu.Unlock()
	if d, exists := user.conns[conn]; exists {
		if user.maxConnCount > 0 {
			user.usedIds[d.id] = false
		}
		delete(user.conns, conn)
		return nil
	}
	return errors.New("connection doesn't exist")
}

func (user *wscUser) cleanup() {
	user.mu.Lock()
	defer user.mu.Unlock()
	for conn := range user.conns {
		conn.Close()
	}
}
