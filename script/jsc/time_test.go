package jsc_test

import (
	"testing"
	"time"

	"github.com/sagernet/sing-box/script/jsc"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func TestTimeToValue(t *testing.T) {
	t.Parallel()
	runtime := goja.New()
	now := time.Now()
	err := runtime.Set("now", jsc.TimeToValue(runtime, now))
	require.NoError(t, err)
	println(runtime.Get("now").String())
}
