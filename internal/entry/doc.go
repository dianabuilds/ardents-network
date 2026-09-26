// Package entry owns the installation-scoped closed Entry sets for the
// Linux protected text Endpoint: exactly two State-current members per
// activated adjacent Role Domain, durably and uniformly selected before
// use, retained through failure and restart until expiry, and revalidated
// without failure-triggered replacement. Its distinct claimed root refuses
// every foreign population through its own marker and allowed-name
// inspection.
//
// The former Invite subsystem - recipient-bound Entry Invite v2 decoding
// and validation, the owner-local recipient identity, the two-slot
// replacement/replay set, the retained attempt/contact journal schema, and
// the `entry recipient`/`entry import` operator commands - was retired by
// ADR-0106. No working-tree code reads, converts, or deletes an existing
// Invite root; its stored bytes remain on disk as unread typed-incompatible
// data, and the retired commands refuse before any effect.
package entry
