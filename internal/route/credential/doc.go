// Package credential owns the closed OHTTP request/profile exchange that
// obtains one membership-level Introduction Transit Grant. It has no Name,
// Target, Descriptor, Publisher, Entry Invite, route, or generic proxy API.
// State selects the issuer identity/profile through its caller; the issuer's
// authorization port owns only current duty and bounded-admission checks.
package credential
