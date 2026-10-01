// Package credential owns the distinct closed-admission permission and blind
// issuer flows. State selects the issuer identity through the Endpoint caller;
// this package has no Transit receiving authorization port, current-duty
// contract, root opener, or signer. The signed Transit profile decoder and
// one-use OHTTP client were retired with the generic transit acquisition chain
// (ADR-0092). It has no Name, Target, Descriptor, Publisher, Entry Invite,
// route, or generic proxy API.
package credential
