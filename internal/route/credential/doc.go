// Package credential owns the retained signed Transit profile decoder and
// one-use OHTTP client, plus the distinct closed-admission permission and blind
// issuer flows. State selects a Transit issuer identity/profile through the
// Endpoint caller; this package has no Transit receiving authorization port,
// current-duty contract, root opener, or signer. It has no Name, Target,
// Descriptor, Publisher, Entry Invite, route, or generic proxy API.
package credential
