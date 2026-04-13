package rule

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIPSetItemString(t *testing.T) {
	// Test single set name
	item, err := NewIPSetItem(nil, []string{"testset"})
	require.NoError(t, err)
	require.Equal(t, "ipset=testset", item.String())

	// Test multiple sets
	item2, err := NewIPSetItem(nil, []string{"set1", "set2", "set3"})
	require.NoError(t, err)
	require.Equal(t, "ipset=[set1 set2 set3]", item2.String())

	// Test many sets (truncated)
	item3, err := NewIPSetItem(nil, []string{"set1", "set2", "set3", "set4"})
	require.NoError(t, err)
	require.Equal(t, "ipset=[set1 set2 set3...]", item3.String())
}

func TestIPSetItemEmptySetNames(t *testing.T) {
	_, err := NewIPSetItem(nil, []string{})
	require.Error(t, err)
	require.Contains(t, err.Error(), "no ipset names provided")
}

func TestIPSetItemDescription(t *testing.T) {
	tests := []struct {
		name     string
		setNames []string
		expected string
	}{
		{
			name:     "single set",
			setNames: []string{"blocklist"},
			expected: "ipset=blocklist",
		},
		{
			name:     "multiple sets",
			setNames: []string{"set1", "set2"},
			expected: "ipset=[set1 set2]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item, err := NewIPSetItem(nil, tt.setNames)
			require.NoError(t, err)
			require.Equal(t, tt.expected, item.String())
		})
	}
}
