package balancer

import (
	"testing"

	"github.com/sagernet/sing-box/option"
)

func TestSelectNodes(t *testing.T) {
	nodes := []*Node{
		{HealthCheckStats: HealthCheckStats{Weighted: 50}},
		{HealthCheckStats: HealthCheckStats{Weighted: 70}},
		{HealthCheckStats: HealthCheckStats{Weighted: 100}},
		{HealthCheckStats: HealthCheckStats{Weighted: 110}},
		{HealthCheckStats: HealthCheckStats{Weighted: 120}},
		{HealthCheckStats: HealthCheckStats{Weighted: 150}},
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
			if got := selectNodes(nodes, nil, tt.expected, tt.baselines); len(got) != tt.want {
				t.Errorf("selectNodes() = %v, want %v", len(got), tt.want)
			}
		})
	}
}
