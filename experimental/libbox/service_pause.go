package libbox

import (
	"sync"
	"time"
)

type servicePauseFields struct {
	pauseAccess sync.Mutex
	pauseTimer  *time.Timer
}

func (s *BoxService) Pause() {
	s.pauseAccess.Lock()
	defer s.pauseAccess.Unlock()
	if s.pauseTimer != nil {
		s.pauseTimer.Stop()
	}
	s.pauseTimer = time.AfterFunc(3*time.Second, s.ResetNetwork)
}

func (s *BoxService) Wake() {
	s.pauseAccess.Lock()
	defer s.pauseAccess.Unlock()
	if s.pauseTimer != nil {
		s.pauseTimer.Stop()
	}
	s.pauseTimer = time.AfterFunc(3*time.Minute, s.ResetNetwork)
}

func (s *BoxService) ResetNetwork() {
	s.instance.Router().ResetNetwork()
}
