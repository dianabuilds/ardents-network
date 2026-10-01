// Package client owns the outgoing closed Route client path: the retained
// Source prefix with its multiplexed lanes, credit accounting and child
// lifetimes, the one-shot bootstrap issuance exchange, recipient inspection,
// and the registration, JOIN, resolution and submission operations that
// borrow a retained prefix.
//
// This package selects no peer on its own: it validates the live State
// selection against Route's shared bootstrap plan contract and dials through
// the Carrier leaf. Route keeps the receiving duties, the outer bridge and
// admission machinery, and the purpose-to-duty assignment table; every
// Route fact this package consumes has its single definition there.
package client
