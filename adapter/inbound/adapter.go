package inbound

type Adapter struct {
	inboundType string
	inboundTag  string
}

func NewAdapter(inboundType string, inboundTag string) Adapter {
	return Adapter{
		inboundType: inboundType,
		inboundTag:  inboundTag,
	}
}

func (a *Adapter) Type() string {
	return a.inboundType
}

func (a *Adapter) Tag() string {
	return a.inboundTag
}
