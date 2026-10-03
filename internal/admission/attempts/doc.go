//go:build linux

// Package attempts owns the Linux Endpoint's bounded durable record of
// potentially spent closed-Route tokens. Mark burns one token attempt before
// presentation and retains ambiguity, replay, and time floors across restart.
// Admission stock owns consumption and Endpoint supplies local authorization;
// this package owns only the durable receipt.
package attempts
