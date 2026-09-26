// Package reachability owns the closed, Instance-signed Service Reachability
// Descriptor, Gateway-local durable Target generation/conflict state, and the
// v3 private recipient proof and exact-publication revision floor. It neither
// selects State peers, acquires Entry, nor creates a Service Connection.
// ADR-0091 retired the former unwired fixed-size OHTTP adapter; ADR-0109
// (F-32) retired the generation-2 v1/v2 Descriptor grammar and refuses a
// stored legacy record with a typed ErrLegacyRecord instead of decoding it.
package reachability
