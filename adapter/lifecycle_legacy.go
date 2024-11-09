package adapter

type LegacyPreStarter interface {
	PreStart() error
}

type LegacyPostStarter interface {
	PostStart() error
}

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
	case StartStatePostStart:
		if postStarter, isPostStarter := starter.(interface {
			PostStart() error
		}); isPostStarter {
			return postStarter.PostStart()
		}
	}
	return nil
}
