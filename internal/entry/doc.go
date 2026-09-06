// Package entry owns the durable, bounded Entry Invite set for the native
// Interactive Route. It retains one owner-local recipient TLS identity and
// validates signed State-referenced recipient-bound Invites against its own
// retained recipient identity at local import and against the presented TLS
// key at Initiator admission, retains one replacement per slot, and
// never chooses a complete Route, transport, or User identity.
package entry
