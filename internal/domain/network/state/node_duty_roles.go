package state

import "errors"

// ErrForwardingRepairUnavailable denotes a history outside the bounded
// Forwarding participation-repair class.
var ErrForwardingRepairUnavailable = errors.New("retained Forwarding producer repair is unavailable")

// The shared repair holds both original leases and binds the sole live fence
// to the current generation. It never repairs spend obligations.
var forwardingRepairRole = nodeDutyRepairRole{
	name:        "forwarding",
	unavailable: ErrForwardingRepairUnavailable,
}

// ErrJoinRepairUnavailable denotes a history outside the bounded JOIN
// participation-repair class.
var ErrJoinRepairUnavailable = errors.New("retained JOIN producer repair is unavailable")

// Each JOIN participant retains its own root and fence.
var joinRepairRole = nodeDutyRepairRole{
	name:        "join",
	unavailable: ErrJoinRepairUnavailable,
}

// ErrIntroductionRepairUnavailable denotes a history outside the bounded
// Introduction participation-repair class.
var ErrIntroductionRepairUnavailable = errors.New("retained Introduction producer repair is unavailable")

// Introduction repair never repairs slot claims or spend obligations.
var introductionRepairRole = nodeDutyRepairRole{
	name:        "introduction",
	unavailable: ErrIntroductionRepairUnavailable,
}

// ErrDescriptorRepairUnavailable denotes a history outside the bounded
// Descriptor participation-repair class.
var ErrDescriptorRepairUnavailable = errors.New("retained Descriptor producer repair is unavailable")

// Descriptor Store stage recovery remains a separate bounded operation.
var descriptorRepairRole = nodeDutyRepairRole{
	name:        "descriptor",
	unavailable: ErrDescriptorRepairUnavailable,
}
