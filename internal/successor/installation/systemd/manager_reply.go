package systemd

// managerUnitReply binds the physical unit/job object reply grammar of this
// connection; the absence-only vocabulary admits no such object. Fixed-purpose
// owners retain their own methods and complete policy. A reply grants no Start.
type managerUnitReply uint8

const (
	sourceUnitReply managerUnitReply = iota
	forwardingUnitReply
	nodeSlotsReply           // Absence-only connection: no unit or job object replies.
	descriptorControlReply   // Fixed root Descriptor Start and failed-start cleanup only.
	descriptorUnitReply      // Fixed Descriptor unit only; no jobs.
	joinUnitReply            // Fixed JOIN unit only; no job replies for stopped installation.
	joinControlReply         // Separate root start/cleanup vocabulary, never stopped/runtime permission.
	introductionUnitReply    // Fixed Introduction unit only; no jobs.
	introductionControlReply // Fixed root Introduction first-start/failed-start cleanup only.
)

func (reply managerUnitReply) valid() bool {
	return reply == introductionControlReply || reply == introductionUnitReply || reply == sourceUnitReply || reply == forwardingUnitReply || reply == nodeSlotsReply || reply == joinUnitReply || reply == joinControlReply || reply == descriptorUnitReply || reply == descriptorControlReply
}

func (reply managerUnitReply) object() string {
	switch reply {
	case introductionUnitReply, introductionControlReply:
		return "/org/freedesktop/systemd1/unit/ardents_2dintroduction_2eservice"
	case sourceUnitReply:
		return "/org/freedesktop/systemd1/unit/ardents_2dsource_2eservice"
	case forwardingUnitReply:
		return "/org/freedesktop/systemd1/unit/ardents_2dforwarding_2eservice"
	case descriptorUnitReply, descriptorControlReply:
		return "/org/freedesktop/systemd1/unit/ardents_2ddescriptor_2eservice"
	case joinUnitReply, joinControlReply:
		return "/org/freedesktop/systemd1/unit/ardents_2djoin_2eservice"
	default:
		return ""
	}
}
