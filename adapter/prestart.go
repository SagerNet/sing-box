package adapter

type PreStarter interface {
	PreStart() error
}

type PostStarter interface {
	PostStart() error
}
