package adapter

func LegacyStart(starter any, stage StartStage) error {
	if lifecycle, isLifecycle := starter.(Lifecycle); isLifecycle {
		return lifecycle.Start(stage)
	}
	switch stage {
	case StartStateInitialize:
		if preStarter, isPreStarter := starter.(interface {
			PreStart() error
		}); isPreStarter {
			return preStarter.PreStart()
		}
	case StartStateStart:
		if starter, isStarter := starter.(interface {
			Start() error
		}); isStarter {
			return starter.Start()
		}
	case StartStateStarted:
		if postStarter, isPostStarter := starter.(interface {
			PostStart() error
		}); isPostStarter {
			return postStarter.PostStart()
		}
	}
	return nil
}

type lifecycleServiceWrapper struct {
	SimpleLifecycle
	name string
}

func NewLifecycleService(service SimpleLifecycle, name string) LifecycleService {
	return &lifecycleServiceWrapper{
		SimpleLifecycle: service,
		name:            name,
	}
}

func (l *lifecycleServiceWrapper) Name() string {
	return l.name
}

func (l *lifecycleServiceWrapper) Start(stage StartStage) error {
	return LegacyStart(l.SimpleLifecycle, stage)
}

func (l *lifecycleServiceWrapper) Close() error {
	return l.SimpleLifecycle.Close()
}
