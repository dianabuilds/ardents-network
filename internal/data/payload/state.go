package payload

func StateRequiresLocalPayload(state string) bool {
	switch state {
	case "available-local", "retained-temporary", "pinned":
		return true
	default:
		return false
	}
}
