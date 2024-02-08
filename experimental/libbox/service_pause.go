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

	s.pauseTimer = time.AfterFunc(time.Minute, s.pause)
}

func (s *BoxService) pause() {
	s.pauseAccess.Lock()
	defer s.pauseAccess.Unlock()

	s.pauseManager.DevicePause()
	_ = s.instance.Router().ResetNetwork()
	s.pauseTimer = nil
}

func (s *BoxService) Wake() {
	s.pauseAccess.Lock()
	defer s.pauseAccess.Unlock()

	if s.pauseTimer != nil {
		s.pauseTimer.Stop()
		s.pauseTimer = nil
		return
	}

	s.pauseManager.DeviceWake()
	_ = s.instance.Router().ResetNetwork()
}
