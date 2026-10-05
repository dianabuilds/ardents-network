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
//
// Retire interrupts physical work. Done signals reader completion; callers
// still call Close to join writers and children and retain terminal failures
// before returning reservations. Lane Finish returns its bounded memory only
// after its operation joins. Shared Budget is one physical principal's memory
// owner; it is independent of Admission allowances and Hosting capacity.
package channel
