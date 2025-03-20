package jsc_test

import (
	"fmt"
	"net/http"
	"reflect"
	"testing"

	"github.com/sagernet/sing-box/script/jsc"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestHeaders(t *testing.T) {
	runtime := goja.New()
	runtime.Set("headers", jsc.HeadersToValue(runtime, http.Header{
		"My-Header": []string{"My-Value1", "My-Value2"},
	}))
	headers := runtime.Get("headers").(*goja.Object).Get("My-Header").(*goja.Object)
	fmt.Println(reflect.ValueOf(headers.Export()).Type().String())
}

func TestBody(t *testing.T) {
	runtime := goja.New()
	_, err := runtime.RunString(`
var responseBody = new Uint8Array([1, 2, 3, 4, 5])
`)
	require.NoError(t, err)
	fmt.Println(reflect.TypeOf(runtime.Get("responseBody").Export()))
}
