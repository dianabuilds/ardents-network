package state

import "errors"

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
