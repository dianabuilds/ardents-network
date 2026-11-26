// Package runtimeevent classifies finite local command observations and reads retained runtime events.
// It owns their shared decoding and finite transition catalogue, not domain policy, runtime authority or SDK lifecycle. Records contain bounded local diagnostic details; export
// projections must still exclude role identity and free-form Node reasons.
package runtimeevent
