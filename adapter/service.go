package adapter

type Service interface {
	Start() error
	Close() error
}

type BoxService interface {
	Service
	Type() string
	Tag() string
}
