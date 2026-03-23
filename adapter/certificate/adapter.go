package certificate

type Adapter struct {
	providerType string
	providerTag  string
}

func NewAdapter(providerType string, providerTag string) Adapter {
	return Adapter{
		providerType: providerType,
		providerTag:  providerTag,
	}
}

func (a *Adapter) Type() string {
	return a.providerType
}

func (a *Adapter) Tag() string {
	return a.providerTag
}
