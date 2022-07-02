package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMatch(t *testing.T) {
	r := require.New(t)
	matcher := NewMatcher([]string{"domain.com"}, []string{"suffix.com", ".suffix.org"})
	r.True(matcher.Match("domain.com"))
	r.False(matcher.Match("my.domain.com"))
	r.True(matcher.Match("suffix.com"))
	r.True(matcher.Match("my.suffix.com"))
	r.False(matcher.Match("suffix.org"))
	r.True(matcher.Match("my.suffix.org"))
}
