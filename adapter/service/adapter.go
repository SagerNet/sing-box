package service

type Adapter struct {
	serviceType string
	serviceTag  string
}

func NewAdapter(serviceType string, serviceTag string) Adapter {
	return Adapter{
		serviceType: serviceType,
		serviceTag:  serviceTag,
	}
}

func (a *Adapter) Type() string {
	return a.serviceType
}

func (a *Adapter) Tag() string {
	return a.serviceTag
}
