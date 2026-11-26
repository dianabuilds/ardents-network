// Package runtimeevent reads the three retained local runtime event formats.
// It owns their shared decoding and category catalogue, not runtime authority
// or instrumentation. Records contain bounded local diagnostic details; export
// projections must still exclude role identity and free-form Node reasons.
package runtimeevent
