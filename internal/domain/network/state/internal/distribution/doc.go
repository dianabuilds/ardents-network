// Package distribution owns the canonical ARDS1D4 journal record and digest.
// It checks the shape of retained cycle, observation and floor fields. State
// alone authenticates the retained generations, admits transitions, commits
// the journal through its durable root and supervises the Source lifetime.
// Decoding supplies storage facts and confers no runtime authority.
package distribution
