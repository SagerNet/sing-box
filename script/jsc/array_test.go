package jsc_test

import (
	"testing"

	"github.com/sagernet/sing-box/script/jsc"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestNewUInt8Array(t *testing.T) {
	runtime := goja.New()
	runtime.Set("hello", jsc.NewUint8Array(runtime, []byte("world")))
	result, err := runtime.RunString("hello instanceof Uint8Array")
	require.NoError(t, err)
	require.True(t, result.ToBoolean())
}
