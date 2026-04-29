package main

import (
	"testing"

	"github.com/sagernet/sing-box/option"

	"github.com/stretchr/testify/require"
)

func TestPrepareOptionsForCheck(t *testing.T) {
	t.Parallel()

	t.Run("override existing log options", func(t *testing.T) {
		t.Parallel()

		logOptions := &option.LogOptions{
			Level:     "trace",
			Timestamp: true,
			Output:    "stderr",
		}
		options := option.Options{
			Log: logOptions,
		}

		prepareOptionsForCheck(&options)

		require.NotNil(t, options.Log)
		require.True(t, options.Log.Disabled)
		require.NotSame(t, logOptions, options.Log)
	})

	t.Run("set log options when absent", func(t *testing.T) {
		t.Parallel()

		options := option.Options{}

		prepareOptionsForCheck(&options)

		require.NotNil(t, options.Log)
		require.True(t, options.Log.Disabled)
	})
}
