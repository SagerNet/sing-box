package balancer

import (
	"testing"

	"github.com/sagernet/sing-box/option"
)

func TestSelectNodes(t *testing.T) {
	nodes := []*Node{
		{RTTStats: RTTStats{Weighted: 50}},
		{RTTStats: RTTStats{Weighted: 70}},
		{RTTStats: RTTStats{Weighted: 100}},
		{RTTStats: RTTStats{Weighted: 110}},
		{RTTStats: RTTStats{Weighted: 120}},
		{RTTStats: RTTStats{Weighted: 150}},
	}
	tests := []struct {
		expected  int
		baselines []option.Duration
		want      int
	}{
		{expected: -1, baselines: nil, want: 1},
		{expected: 0, baselines: nil, want: 1},
		{expected: 1, baselines: nil, want: 1},
		{expected: 9999, baselines: nil, want: len(nodes)},
		{expected: 0, baselines: []option.Duration{80, 100}, want: 2},
		{expected: 2, baselines: []option.Duration{50, 100}, want: 2},
		{expected: 3, baselines: []option.Duration{50, 100, 150}, want: 5},
		{expected: 9999, baselines: []option.Duration{50, 100, 150}, want: len(nodes)},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			if got := selectNodes(nodes, tt.expected, tt.baselines); len(got) != tt.want {
				t.Errorf("selectNodes() = %v, want %v", len(got), tt.want)
			}
		})
	}
}
