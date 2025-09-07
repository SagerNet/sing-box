package badversion

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompareVersion(t *testing.T) {
	t.Parallel()
	require.Equal(t, "1.3.0-beta.1", Parse("v1.3.0-beta1").String())
	require.Equal(t, "1.3-beta1", Parse("v1.3.0-beta.1").BadString())
	require.True(t, Parse("1.3.0").GreaterThan(Parse("1.3-beta1")))
	require.True(t, Parse("1.3.0").GreaterThan(Parse("1.3.0-beta1")))
	require.True(t, Parse("1.3.0-beta1").GreaterThan(Parse("1.3.0-alpha1")))
	require.True(t, Parse("1.3.1").GreaterThan(Parse("1.3.0")))
	require.True(t, Parse("1.4").GreaterThan(Parse("1.3")))
}
