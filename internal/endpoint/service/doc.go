//go:build linux

// Package service owns the protected Service stream mechanism: the checked
// directional workload contract, the TLS attachment establishment with its
// exported continuity commitment, the in-process Application half-close pair,
// the resource ledger, and the bounded native Connection lifecycle around one
// authenticated byte stream.
//
// Binding authority, role orchestration, Route recovery, and job identity
// remain in the Endpoint root behind the Binding seam; this package cannot
// establish reachability or reinterpret a workload on its own.
package service
