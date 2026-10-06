// Package channel owns portable Route framing, bounded principal memory,
// scheduling, credit and byte accounting, parent exchanges and joined physical
// retirement. It consumes an already selected ordered transport and observes
// caller-supplied currentness; it grants no Network or Admission authority.
//
// New starts the sole reader after the caller's accepted handshake. Prepare
// and PrepareJoined defer that transfer until Start, so refusal can close an
// unused preparation. Callers supply a nonnil context, connection and Budget,
// their original finite deadline and already accepted byte allowance. Handlers
// are captured before reading starts; they own domain decisions and return
// genuine owner-backed results to the physical channel.
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
package channel
