package conf

import "testing"

func TestIsRemote(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"http://example.com", true},
		{"https://example.com/config.json", true},
		{"config.json", false},
		{"path/to/config.json", false},
		{"/path/to/config.json", false},
		{`d:\config.json`, false},
	}
	for _, tt := range tests {
		if got := isRemote(tt.path); got != tt.want {
			t.Errorf("isRemote(%s) = %v, want %v", tt.path, got, tt.want)
		}
	}
}
