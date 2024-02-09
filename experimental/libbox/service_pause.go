package libbox

import (
	"sync"
	"time"

	"github.com/sagernet/sing-box/log"
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
		log.Debug("pause() reconfigured")
	}

	s.pauseTimer = time.AfterFunc(time.Minute, s.pause)
}

func (s *BoxService) pause() {
	s.pauseAccess.Lock()
	defer s.pauseAccess.Unlock()

	s.pauseManager.DevicePause()
	_ = s.instance.Router().ResetNetwork()
	s.pauseTimer = nil

	log.Debug("pause()")
}

func (s *BoxService) Wake() {
	s.pauseAccess.Lock()
	defer s.pauseAccess.Unlock()

	if s.pauseTimer != nil {
		s.pauseTimer.Stop()
		s.pauseTimer = nil
		log.Debug("pause() ignored")
		return
	}

	s.pauseManager.DeviceWake()
	_ = s.instance.Router().ResetNetwork()
	log.Debug("wake()")
}
