// Package admission defines permission and token request contracts and checks
// externally selected facts. Quota, issuance, holder stock and receiver spending
// own their separate state transitions in cohesive child packages.
//
// Allocation decides the finite permission. Issuer composes quota and the
// original private issuance/result owner. Stock holds and presents tokens;
// attempts retains those presentations. Receiving verifies and reserves capacity
// before committing through spending and accepting. The local console supplies
// transport for the holder and receiver. It leaves those effects with them.
package admission
