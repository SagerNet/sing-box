package adapter

type Hook interface {
	PreStart() error
	PostStart() error
	PreStop() error
	PostStop() error
}
