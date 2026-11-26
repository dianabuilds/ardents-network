// Package pathgrammar provides shared, purely lexical path validation.
// Strict and DisjointRoots validate host paths, while LiteralPOSIXPath checks
// the portable literal-path grammar used in system unit declarations.
//
// It is a pure mechanism leaf, not a root or custody owner. The predicates
// report a boolean; the package holds no root,
// marker, schema, authority or lifecycle of its own, performs no filesystem
// observation and no relative-to-absolute conversion. Each consuming package
// keeps its distinct refusal prose and any additional root, NUL-rune or length
// bounds it genuinely needs, composed around this predicate.
//
// DisjointRoots compares path spellings, not filesystem identities; it does
// not resolve symbolic links or account for filesystem-specific case folding.
package pathgrammar
