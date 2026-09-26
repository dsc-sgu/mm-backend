package git

import "github.com/gobwas/glob"

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
		g, err := glob.Compile(pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, g)
	}
	return compiled
}

// MatchAny reports whether name matches any of the compiled patterns.
// Patterns are compiled with no separator characters, so '*' and '?' match
// '/' too — the same rule the pre-receive hook applies via a shell case
// statement (see WritePreReceiveHook), rather than filepath.Match's, where
// '*' stops at '/'. This keeps the accept/reject decision for a submission
// identical whether it arrives over SSH git push or through the web zip
// upload (see the "Mask gate" in internal/attempts/README.md).
func (c CompiledPatterns) MatchAny(name string) bool {
	for _, g := range c {
		if g.Match(name) {
			return true
		}
	}
	return false
}
