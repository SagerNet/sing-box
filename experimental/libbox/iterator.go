package libbox

import "github.com/sagernet/sing/common"

type StringIterator interface {
	Next() string
	HasNext() bool
}

var _ StringIterator = (*iterator[string])(nil)

type iterator[T any] struct {
	values []T
}

func newIterator[T any](values []T) *iterator[T] {
	return &iterator[T]{values}
}

func (i *iterator[T]) Next() T {
	if len(i.values) == 0 {
		return common.DefaultValue[T]()
	}
	nextValue := i.values[0]
	i.values = i.values[1:]
	return nextValue
}

func (i *iterator[T]) HasNext() bool {
	return len(i.values) > 0
}
