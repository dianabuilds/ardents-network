// Package reachability owns the closed, Instance-signed Service Reachability
// Descriptor, Gateway-local durable Target generation/conflict state, and the
// v3 private recipient proof and exact-publication revision floor. It neither
// selects State peers, acquires Entry, nor creates a Service Connection.
// ADR-0091 retired the former unwired fixed-size OHTTP adapter.
package reachability
