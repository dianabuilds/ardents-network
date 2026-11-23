// Package pathgrammar owns the one strict path-validation grammar shared by
// the maintained owners (G7, #772): a path is admitted only when it is already
// absolute and clean on the host platform.
//
// It is a pure mechanism leaf, not a root or custody owner. The single
// predicate takes a caller path and reports a boolean; it holds no root,
// marker, schema, authority or lifecycle of its own, performs no filesystem
// observation and no relative-to-absolute conversion. Each consuming package
// keeps its distinct refusal prose and any additional root, NUL-rune or length
// bounds it genuinely needs, composed around this predicate.
//
// The grammar replaces the formerly repeated inline
// !filepath.IsAbs(x) || filepath.Clean(x) != x checks whose variants drifted
// across owners. Deliberate boundaries remain separate: the POSIX declaration
// wire grammar in endpoint/plan uses the platform-independent path package,
// the Application zone stays pure, and configured-root resolution operations
// (which legitimately absolutize a relative root) are not this validation
// grammar. Network State configuration validation refuses relative and
// non-clean paths instead of silently absolutizing them (the one deliberate
// behavior tightening in #772; every production caller already supplies
// absolute clean paths).
//
// Responsibility clusters and their tests:
//   - strict absolute/clean predicate: strict_path.go (strict_path_test.go)
package pathgrammar
