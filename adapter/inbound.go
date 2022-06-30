package adapter

type Inbound interface {
	Service
	Type() string
	Tag() string
}
