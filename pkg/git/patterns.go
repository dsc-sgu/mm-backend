package git

import (
	"strings"

	"github.com/gobwas/glob"
)

// braceEscaper makes braces literal, as they were when the pre-receive hook
// matched patterns with a shell case statement, which has no "{a,b}"
// alternation.
var braceEscaper = strings.NewReplacer("{", `\{`, "}", `\}`)

// CompiledPatterns is a set of glob patterns compiled once via
// CompilePatterns, for matching many names without recompiling on every
// call — useful when a caller checks the same pattern set against a large
// list of paths (e.g. PushAttempt deciding what a resubmission should
// prune).
type CompiledPatterns []*glob.Pattern

// CompilePatterns compiles patterns for repeated matching via
// CompiledPatterns.MatchAny. A pattern that fails to compile is skipped.
func CompilePatterns(patterns []string) CompiledPatterns {
	compiled := make(CompiledPatterns, 0, len(patterns))
	for _, pattern := range patterns {
		g, err := glob.Compile(braceEscaper.Replace(pattern))
		if err != nil {
			continue
		}
		compiled = append(compiled, g)
	}
	return compiled
}

// MatchAny reports whether name matches any of the compiled patterns.
// Patterns are compiled with no separator characters, so '*' and '?' match
// '/' too, unlike filepath.Match, where '*' stops at '/'. Both the SSH push
// and the web zip upload are checked with it (see the "Mask gate" in
// internal/attempts/README.md).
func (c CompiledPatterns) MatchAny(name string) bool {
	for _, g := range c {
		if g.Match(name) {
			return true
		}
	}
	return false
}
