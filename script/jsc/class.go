package jsc

import (
	"time"

	"github.com/sagernet/sing/common"

	"github.com/dop251/goja"
)

type Module interface {
	Runtime() *goja.Runtime
}

type Class[M Module, C any] interface {
	Module() M
	Runtime() *goja.Runtime
	DefineField(name string, getter func(this C) any, setter func(this C, value goja.Value))
	DefineMethod(name string, method func(this C, call goja.FunctionCall) any)
	DefineStaticMethod(name string, method func(c Class[M, C], call goja.FunctionCall) any)
	DefineConstructor(constructor func(c Class[M, C], call goja.ConstructorCall) C)
	ToValue() goja.Value
	New(instance C) *goja.Object
	Prototype() *goja.Object
	Is(value goja.Value) bool
	As(value goja.Value) C
}

func GetClass[M Module, C any](runtime *goja.Runtime, exports *goja.Object, className string) Class[M, C] {
	objectValue := exports.Get(className)
	if objectValue == nil {
		panic(runtime.NewTypeError("Missing class: " + className))
	}
	object, isObject := objectValue.(*goja.Object)
	if !isObject {
		panic(runtime.NewTypeError("Invalid class: " + className))
	}
	classObject, isClass := object.Get("_class").(*goja.Object)
	if !isClass {
		panic(runtime.NewTypeError("Invalid class: " + className))
	}
	class, isClass := classObject.Export().(Class[M, C])
	if !isClass {
		panic(runtime.NewTypeError("Invalid class: " + className))
	}
	return class
}

type goClass[M Module, C any] struct {
	m           M
	prototype   *goja.Object
	constructor goja.Value
}

func NewClass[M Module, C any](module M) Class[M, C] {
	class := &goClass[M, C]{
		m:         module,
		prototype: module.Runtime().NewObject(),
	}
	clazz := module.Runtime().ToValue(class).(*goja.Object)
	clazz.Set("toString", module.Runtime().ToValue(func(call goja.FunctionCall) goja.Value {
		return module.Runtime().ToValue("[sing-box Class]")
	}))
	class.prototype.DefineAccessorProperty("_class", class.Runtime().ToValue(func(call goja.FunctionCall) goja.Value { return clazz }), nil, goja.FLAG_FALSE, goja.FLAG_TRUE)
	return class
}

func (c *goClass[M, C]) Module() M {
	return c.m
}

func (c *goClass[M, C]) Runtime() *goja.Runtime {
	return c.m.Runtime()
}

func (c *goClass[M, C]) DefineField(name string, getter func(this C) any, setter func(this C, value goja.Value)) {
	var (
		getterValue goja.Value
		setterValue goja.Value
	)
	if getter != nil {
		getterValue = c.Runtime().ToValue(func(call goja.FunctionCall) goja.Value {
			this, isThis := call.This.Export().(C)
			if !isThis {
				panic(c.Runtime().NewTypeError("Illegal this value: " + call.This.ExportType().String()))
			}
			return c.toValue(getter(this), goja.Null())
		})
	}
	if setter != nil {
		setterValue = c.Runtime().ToValue(func(call goja.FunctionCall) goja.Value {
			this, isThis := call.This.Export().(C)
			if !isThis {
				panic(c.Runtime().NewTypeError("Illegal this value: " + call.This.String()))
			}
			setter(this, call.Argument(0))
			return goja.Undefined()
		})
	}
	c.prototype.DefineAccessorProperty(name, getterValue, setterValue, goja.FLAG_FALSE, goja.FLAG_TRUE)
}

func (c *goClass[M, C]) DefineMethod(name string, method func(this C, call goja.FunctionCall) any) {
	methodValue := c.Runtime().ToValue(func(call goja.FunctionCall) goja.Value {
		this, isThis := call.This.Export().(C)
		if !isThis {
			panic(c.Runtime().NewTypeError("Illegal this value: " + call.This.String()))
		}
		return c.toValue(method(this, call), goja.Undefined())
	})
	c.prototype.Set(name, methodValue)
	if name == "entries" {
		c.prototype.DefineDataPropertySymbol(goja.SymIterator, methodValue, goja.FLAG_TRUE, goja.FLAG_FALSE, goja.FLAG_TRUE)
	}
}

func (c *goClass[M, C]) DefineStaticMethod(name string, method func(c Class[M, C], call goja.FunctionCall) any) {
	c.prototype.Set(name, c.Runtime().ToValue(func(call goja.FunctionCall) goja.Value {
		return c.toValue(method(c, call), goja.Undefined())
	}))
}

func (c *goClass[M, C]) DefineConstructor(constructor func(c Class[M, C], call goja.ConstructorCall) C) {
	constructorObject := c.Runtime().ToValue(func(call goja.ConstructorCall) *goja.Object {
		value := constructor(c, call)
		object := c.toValue(value, goja.Undefined()).(*goja.Object)
		object.SetPrototype(call.This.Prototype())
		return object
	}).(*goja.Object)
	constructorObject.SetPrototype(c.prototype)
	c.prototype.DefineDataProperty("constructor", constructorObject, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)
	c.constructor = constructorObject
}

func (c *goClass[M, C]) toValue(rawValue any, defaultValue goja.Value) goja.Value {
	switch value := rawValue.(type) {
	case nil:
		return defaultValue
	case time.Time:
		return TimeToValue(c.Runtime(), value)
	default:
		return c.Runtime().ToValue(value)
	}
}

func (c *goClass[M, C]) ToValue() goja.Value {
	if c.constructor == nil {
		constructorObject := c.Runtime().ToValue(func(call goja.ConstructorCall) *goja.Object {
			panic(c.Runtime().NewTypeError("Illegal constructor call"))
		}).(*goja.Object)
		constructorObject.SetPrototype(c.prototype)
		c.prototype.DefineDataProperty("constructor", constructorObject, goja.FLAG_FALSE, goja.FLAG_FALSE, goja.FLAG_FALSE)
		c.constructor = constructorObject
	}
	return c.constructor
}

func (c *goClass[M, C]) New(instance C) *goja.Object {
	object := c.Runtime().ToValue(instance).(*goja.Object)
	object.SetPrototype(c.prototype)
	return object
}

func (c *goClass[M, C]) Prototype() *goja.Object {
	return c.prototype
}

func (c *goClass[M, C]) Is(value goja.Value) bool {
	object, isObject := value.(*goja.Object)
	if !isObject {
		return false
	}
	prototype := object.Prototype()
	for prototype != nil {
		if prototype == c.prototype {
			return true
		}
		prototype = prototype.Prototype()
	}
	return false
}

func (c *goClass[M, C]) As(value goja.Value) C {
	object, isObject := value.(*goja.Object)
	if !isObject {
		return common.DefaultValue[C]()
	}
	if !c.Is(object) {
		return common.DefaultValue[C]()
	}
	return object.Export().(C)
}
