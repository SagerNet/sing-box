package libbox

import (
	"time"

	C "github.com/sagernet/sing-box/constant"
)

type iOSPauseFields struct {
	endPauseTimer *time.Timer
}

func (s *BoxService) Pause() {
	s.pauseManager.DevicePause()
	if !C.IsIos {
		s.instance.Router().ResetNetwork()
	} else {
		if s.endPauseTimer == nil {
			s.endPauseTimer = time.AfterFunc(time.Minute, s.pauseManager.DeviceWake)
		} else {
			s.endPauseTimer.Reset(time.Minute)
		}
	}
}

func (s *BoxService) Wake() {
	if !C.IsIos {
		s.pauseManager.DeviceWake()
		s.instance.Router().ResetNetwork()
	}
}

func (s *BoxService) ResetNetwork() {
	s.instance.Router().ResetNetwork()
}

func (s *BoxService) UpdateWIFIState() {
	s.instance.Network().UpdateWIFIState()
}
