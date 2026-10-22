// Package source implements the private, finite Direct-Origin Source
// transport used to acquire and redistribute authenticated Network State. It
// owns bounded network-scoped request, response, and private bundle framing with pinned mutual
// TLS, but does not make the transport a public wire protocol or decide whether
// state is acceptable. Its pure local serving-declaration codec is shared by
// command composition and Installation's future-input matching; decoding grants
// neither native preflight admission nor runtime authority.
// ValidateServingCredential shares the existing pure finite Ed25519 chain/key
// agreement policy with Installation's explicit runtime credential delivery;
// it opens no Plan, clock, State, private signer or transport.
// InspectLatest owns one bounded authenticated diagnostic contact, using the
// same complete one-source trust checks, with no acquisition Plan, retry,
// independence claim or accepted State. Its real canonical Control consumer
// retains original local inputs and erases client key copies only after join.
// ObservePublicContact separately checks one public Source/client declaration
// for offline installation-input preparation. It opens no transport and grants
// neither the two-source acquisition Plan nor State/bootstrap authority.
package source
