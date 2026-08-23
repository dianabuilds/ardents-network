// Package entry owns the durable, bounded Entry Invite set for the native
// Interactive Route. It validates signed State-referenced Invites at both
// local import and Initiator admission, retains one replacement per slot, and
// never chooses a complete Route, transport, or User identity.
package entry
