package adapter

type PreStarter interface {
	PreStart() error
}

func PreStart(starter any) error {
	if preService, ok := starter.(PreStarter); ok {
		err := preService.PreStart()
		if err != nil {
			return err
		}
	}
	return nil
}
