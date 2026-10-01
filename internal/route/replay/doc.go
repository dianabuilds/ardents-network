// Package replay owns the receiving duty's durable token-spend and
// Introduction-slot replay floors. One exclusive lease binds both files to
// the same Network, profile, Node, and duty generation. A token is recorded
// before Route admits work; a slot is recorded before registration succeeds.
// This package selects no Carrier, peer, Service, or Application authority.
package replay
