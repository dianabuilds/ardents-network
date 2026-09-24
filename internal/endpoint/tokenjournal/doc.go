// Package tokenjournal owns the Linux Endpoint's bounded durable record of
// potentially spent closed-Route tokens. Mark burns one token attempt before
// presentation and retains ambiguity, replay, and time floors across restart.
// Endpoint owns stock and authorization; this package owns only the receipt.
package tokenjournal
