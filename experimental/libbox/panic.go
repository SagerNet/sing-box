package libbox

// https://github.com/golang/go/issues/46893
// TODO: remove after `bulkBarrierPreWrite: unaligned arguments` fixed

type StringBox struct {
	Value string
}

func wrapString(value string) *StringBox {
	return &StringBox{Value: value}
}
