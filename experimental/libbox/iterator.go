package libbox

import "github.com/sagernet/sing/common"

type StringIterator interface {
	Len() int32
	HasNext() bool
	Next() string
}

var _ StringIterator = (*iterator[string])(nil)

type iterator[T any] struct {
	values []T
}

func newIterator[T any](values []T) *iterator[T] {
	return &iterator[T]{values}
}

//go:noinline
func newPtrIterator[T any](values []T) *iterator[*T] {
	return &iterator[*T]{common.Map(values, func(value T) *T { return &value })}
}

func (i *iterator[T]) Len() int32 {
	return int32(len(i.values))
}

func (i *iterator[T]) HasNext() bool {
	return len(i.values) > 0
}

func (i *iterator[T]) Next() T {
	if len(i.values) == 0 {
		return common.DefaultValue[T]()
	}
	nextValue := i.values[0]
	i.values = i.values[1:]
	return nextValue
}

type abstractIterator[T any] interface {
	Next() T
	HasNext() bool
}

func iteratorToArray[T any](iterator abstractIterator[T]) []T {
	if iterator == nil {
		return nil
	}
	var values []T
	for iterator.HasNext() {
		values = append(values, iterator.Next())
	}
	return values
}
