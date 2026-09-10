// Package entry owns the durable, bounded Entry Invite set for the native
// Interactive Route. It retains one owner-local recipient TLS identity and
// validates signed State-referenced recipient-bound Invites against its own
// retained recipient identity at local import and against the presented TLS
// key at Initiator admission, retains one replacement per slot, and
// never chooses a complete Route, transport, or User identity.
//
// The closed successor additionally owns installation-scoped two-member Entry
// sets for each activated adjacent Role Domain. Their random selection is
// durable before use and remains fixed through failure/restart until expiry.
// A distinct claimed root refuses legacy Invite state pending explicit migration;
// it carries no holder identity, credential, context, destination or transport.
package entry
