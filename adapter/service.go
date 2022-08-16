package adapter

type Service interface {
	Start() error
	Close() error
}
