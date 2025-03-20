package jsc

import "github.com/dop251/goja"

type Iterator[M Module, T any] struct {
	c      Class[M, *Iterator[M, T]]
	values []T
	block  func(this T) any
}

func NewIterator[M Module, T any](class Class[M, *Iterator[M, T]], values []T, block func(this T) any) goja.Value {
	return class.New(&Iterator[M, T]{class, values, block})
}

func CreateIterator[M Module, T any](module M) Class[M, *Iterator[M, T]] {
	class := NewClass[M, *Iterator[M, T]](module)
	class.DefineMethod("next", (*Iterator[M, T]).next)
	class.DefineMethod("toString", (*Iterator[M, T]).toString)
	return class
}

func (i *Iterator[M, T]) next(call goja.FunctionCall) any {
	result := i.c.Runtime().NewObject()
	if len(i.values) == 0 {
		result.Set("done", true)
	} else {
		result.Set("done", false)
		result.Set("value", i.block(i.values[0]))
		i.values = i.values[1:]
	}
	return result
}

func (i *Iterator[M, T]) toString(call goja.FunctionCall) any {
	return "[sing-box Iterator]"
}
