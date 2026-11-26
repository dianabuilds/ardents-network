// Package runtimeevent reads retained local runtime events and current Node/Source command reports.
// It owns their shared decoding and finite transition catalogue, not runtime authority
// or instrumentation. Records contain bounded local diagnostic details; export
// projections must still exclude role identity and free-form Node reasons.
package runtimeevent
