package adapter

import E "github.com/sagernet/sing/common/exceptions"

type SimpleLifecycle interface {
	Start() error
	Close() error
}

type StartStage uint8

const (
	StartStateInitialize StartStage = iota
	StartStateStart
	StartStatePostStart
	StartStateStarted
)

var ListStartStages = []StartStage{
	StartStateInitialize,
	StartStateStart,
	StartStatePostStart,
	StartStateStarted,
}

func (s StartStage) String() string {
	switch s {
	case StartStateInitialize:
		return "initialize"
	case StartStateStart:
		return "start"
	case StartStatePostStart:
		return "post-start"
	case StartStateStarted:
		return "finish-start"
	default:
		panic("unknown stage")
	}
}

type Lifecycle interface {
	Start(stage StartStage) error
	Close() error
}

type LifecycleService interface {
	Name() string
	Lifecycle
}

func Start(stage StartStage, services ...Lifecycle) error {
	for _, service := range services {
		err := service.Start(stage)
		if err != nil {
			return err
		}
	}
	return nil
}

func StartNamed(stage StartStage, services []LifecycleService) error {
	for _, service := range services {
		err := service.Start(stage)
		if err != nil {
			return E.Cause(err, stage.String(), " ", service.Name())
		}
	}
	return nil
}
