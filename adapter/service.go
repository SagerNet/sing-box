package adapter

import "io"

type Starter interface {
	Start() error
}

type Service interface {
	Starter
	io.Closer
}
