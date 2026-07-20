package enforcement

import nodeapi "ardents/internal/node/api"

func Snapshot(state, reason string) nodeapi.PartSnapshot {
	return nodeapi.PartSnapshot{State: state, Reason: reason}
}
