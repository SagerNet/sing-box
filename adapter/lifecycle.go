package adapter

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

func (s StartStage) Action() string {
	switch s {
	case StartStateInitialize:
		return "initialize"
	case StartStateStart:
		return "start"
	case StartStatePostStart:
		return "post-start"
	case StartStateStarted:
		return "start-after-started"
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
