// Package endpoint authenticates an Endpoint program and generation pair
// through independent Enrollment and Release owners. Private proofs, original
// initial pin and immutable inventory cannot be recreated from detached facts.
// It compares retained generation facts with fresh verifier floors without
// turning those facts into a proof. It owns no installed root, process,
// transaction or deployment authority.
package endpoint
