package git

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCompiledPatternsMatchAny(t *testing.T) {
	patterns := CompilePatterns([]string{"a/*", "*.go"})
	for name, want := range map[string]bool{
		"a/x":       true,
		"a/b/c.txt": true, // '*' matches '/', like the pre-receive hook's case
		"x.go":      true,
		"dir/x.go":  true,
		"b/x.txt":   false,
		"a":         false,
	} {
		require.Equal(t, want, patterns.MatchAny(name), name)
	}
}

func TestCompilePatternsSkipsInvalid(t *testing.T) {
	patterns := CompilePatterns([]string{"[", "*.go"})
	require.Len(t, patterns, 1)
	require.True(t, patterns.MatchAny("x.go"))
}

func TestEmptyCompiledPatternsMatchNothing(t *testing.T) {
	require.False(t, CompilePatterns(nil).MatchAny("x.go"))
}
