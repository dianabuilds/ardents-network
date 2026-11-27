// Package channel owns portable Route framing, bounded principal memory,
// scheduling, credit and byte accounting, parent exchanges and joined physical
// retirement. It consumes an already selected ordered transport and observes
// caller-supplied currentness; it grants no Network or Admission authority.
// MinDeadline is the one shared pure setup/cleanup bound for every Route
// owner: the earlier original deadline wins, and equal deadlines keep the
// first argument's exact reading. It grants no deadline authority.
//
// New starts the sole reader after the caller's accepted handshake. Prepare
// and PrepareJoined defer that transfer until Start, so refusal can close an
// unused preparation. Callers supply a nonnil context, connection and Budget,
// their original finite deadline and already accepted byte allowance. Handlers
// are captured before reading starts; they own domain decisions and return
// genuine owner-backed results to the physical channel.
// Both OPEN directions share 256 ordinary child slots and two additional slots
// reserved for Issuer, Reachability, Introduction and Submission. Controls use
// ordinary slots first. Each original claim lasts through joined Lane Finish;
// the extra reserve never bypasses the shared principal's 1024-child bound.
// PrepareOpen binds child capacity synchronously outside the framing lock,
// before pipelined handshake input. ConstrainQueues applies a separate shared
// child-group queue without replacing principal accounting; it includes queued
// output headers/control and keeps finite CLOSE space until physical finish.
// ConstrainTraffic counts complete child input/output, the original OPEN and
// prepaid CLOSE in both directions. It binds the actual operation's fast output
// admission before physical writes. A private witness distinguishes refusal
// before output from an attempted physical failure; only the former can retire
// the limited child while preserving the ordinary Carrier and its siblings.
//
// Retire interrupts physical work. Done signals reader completion; callers
// still call Close to join writers and children and retain terminal failures
// before returning reservations. Lane Finish returns its bounded memory only
// after its operation joins. Shared Budget is one physical principal's memory
// owner; it is independent of Admission allowances and Hosting capacity.
// Close also joins physical-return callbacks already started by Lane Finish.
// FinishRole completes an outgoing nested forwarding role after its borrowers
// join: inner half-close, lower EOF, reverse reader and exact lower CLOSE(0).
// It shares the original one-second cleanup horizon and retains cancellation
// and all started physical failures. A reverse EOF without that peer terminal
// cannot discharge retirement. Physical Close still owns join and returns.
//
// Start at child_opening.go for outgoing OPEN and the dedicated JOIN lane.
// The reader in session.go checks inbound frames and hands incoming OPEN to
// the same slot owner in child_capacity.go. Both directions use the original
// Session lock and principal Budget. Lane.Read and Lane.Write in lane.go own
// queued bytes and consumption. frame_schedule.go orders the writers, and
// physical_output.go debits a complete frame and grants CREDIT when its write
// starts. parent_exchange.go owns replenishment under the same byte account.
// session_retirement.go seals, interrupts, joins and retains the exact physical
// result before capacity returns. role_retirement.go adds the exact peer
// terminal required by a nested role. The session and lane close tests exercise
// these joins with held writers and late physical failures.
package channel
