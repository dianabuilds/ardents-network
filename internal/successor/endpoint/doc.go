//go:build linux

// Package endpoint owns one original installed participant: Startup, local
// Execution, State, permission exchange, shared funding parents and IPC children.
// It joins children before returning their original roots. Domain authority and
// policy remain with Installation, Execution, Network, Admission and Service owners.
// Explicit offline first history preparation authenticates signed State/profile
// and initializes participation, Route selection and Stock; ordinary runtime
// opens retained only.
// The only command consumer is ardents-next; schemas are bound Installation input,
// while arguments, diagnostics and process exits remain with that command.
package endpoint
