package state

import "errors"

// ErrForwardingRepairUnavailable denotes a history outside the bounded
// Forwarding participation-repair class.
var ErrForwardingRepairUnavailable = errors.New("retained Forwarding producer repair is unavailable")

// Forwarding repair is the shared node-duty mechanism bound to the Forwarding
// role. It holds both existing leases until the native adapter's final
// observation and the one participation commit; it never starts a State, joins
// an imaginary completion, repairs floors or resumes an interrupted wave. The
// original Node producer identity is random, so the repair discovers the sole
// live node-duty fence and binds it to the current generation. Spend
// obligations are never repairable here.
var forwardingRepairRole = nodeDutyRepairRole{
	name:        "forwarding",
	unavailable: ErrForwardingRepairUnavailable,
}
