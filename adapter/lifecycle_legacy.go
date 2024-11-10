package adapter

func LegacyStart(starter any, stage StartStage) error {
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
	Service
	name string
}

func NewLifecycleService(service Service, name string) LifecycleService {
	return &lifecycleServiceWrapper{
		Service: service,
		name:    name,
	}
}

func (l *lifecycleServiceWrapper) Name() string {
	return l.name
}

func (l *lifecycleServiceWrapper) Start(stage StartStage) error {
	return LegacyStart(l.Service, stage)
}

func (l *lifecycleServiceWrapper) Close() error {
	return l.Service.Close()
}
