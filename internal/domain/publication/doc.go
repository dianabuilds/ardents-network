// Package publication verifies public Service Credential and Publication proofs.
// It owns the canonical v3 Service request and response, unsigned Credential
// and Publication transcripts, and verification used by Custody and Instance.
// Encoding and verification grant no approval or live authority.
// It owns delegation and capability checks, not Authority issuance, private
// Instance material, registration, Store acknowledgement or live readiness.
package publication
