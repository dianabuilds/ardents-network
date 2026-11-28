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

// ErrJoinRepairUnavailable denotes a history outside the bounded JOIN
// participation-repair class.
var ErrJoinRepairUnavailable = errors.New("retained JOIN producer repair is unavailable")

// JOIN repair is the shared node-duty mechanism bound to the JOIN role. It
// holds both existing leases until the native adapter's final observation and
// the one participation commit; it never starts a State, joins an imaginary
// completion, repairs floors or resumes an interrupted wave. The original Node
// producer identity is random, so the repair discovers the sole live node-duty
// fence and binds it to the current generation. Each JOIN pair participant
// owns its own participation root and its own sole fence; repair is
// per-invocation, never a shared pair effect. Spend obligations are never
// repairable here.
var joinRepairRole = nodeDutyRepairRole{
	name:        "join",
	unavailable: ErrJoinRepairUnavailable,
}

// ErrIntroductionRepairUnavailable denotes a history outside the bounded
// Introduction participation-repair class.
var ErrIntroductionRepairUnavailable = errors.New("retained Introduction producer repair is unavailable")

// Introduction repair is the shared node-duty mechanism bound to the
// Introduction role. It holds both existing leases until the native adapter's
// final observation and the one participation commit; it never starts a State,
// joins an imaginary completion, repairs floors or resumes an interrupted wave.
// Unlike Source, the original Node producer identity is random, so the repair
// discovers the sole live node-duty fence and binds it to the current
// generation. Slot claims and spend obligations are never repairable here.
var introductionRepairRole = nodeDutyRepairRole{
	name:        "introduction",
	unavailable: ErrIntroductionRepairUnavailable,
}

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
