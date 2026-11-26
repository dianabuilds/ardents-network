package networkdraft

const MaximumPlanBytes = 8 << 20

// IsPreparation recognizes the fixed public preparation purposes.
func IsPreparation(operation string) bool {
	return operation == "prepare-initial-node-record" || operation == "prepare-initial-epoch" || operation == "prepare-closed-profile"
}

// Prepare builds exact detached signing bytes from a public plan.
func Prepare(operation string, raw []byte) (string, []byte, error) {
	switch operation {
	case "prepare-initial-node-record":
		_, message, err := NodeRecord(raw)
		return "node-record.signing-input", message, err
	case "prepare-initial-epoch":
		_, message, err := Epoch(raw)
		return "epoch.unsigned", message, err
	case "prepare-closed-profile":
		_, message, err := ClosedProfile(raw)
		return "profile.unsigned", message, err
	default:
		return "", nil, ErrUnavailable
	}
}
