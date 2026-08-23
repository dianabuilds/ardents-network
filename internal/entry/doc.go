// Package entry owns the durable, bounded Entry Invite set for the native
// Interactive Route. It validates signed State-referenced Invites, retains one
// replacement per slot, and never chooses a complete Route or transport.
package entry
