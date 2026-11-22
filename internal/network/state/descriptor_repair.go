package state

import "errors"

// ErrDescriptorRepairUnavailable denotes a history outside the bounded
// Descriptor participation-repair class.
var ErrDescriptorRepairUnavailable = errors.New("retained Descriptor producer repair is unavailable")

// Descriptor repair is the shared node-duty mechanism bound to the Descriptor
// role. It holds both existing leases until the native adapter's final
// observation and the one participation commit; it never starts a State, joins
// an imaginary completion, repairs floors or resumes an interrupted wave. The
// original Node producer identity is random, so the repair discovers the sole
// live node-duty fence and binds it to the current generation. Descriptor
// claims the same node-duty participation class as Forwarding/JOIN; its separate
// Store root carries no durable participation fence, and the bounded Store
// stage recovery is a distinct reachability mechanism the native adapter
// invokes under these same proofs. Publication floors are never repairable.
var descriptorRepairRole = nodeDutyRepairRole{
	name:        "descriptor",
	unavailable: ErrDescriptorRepairUnavailable,
}
