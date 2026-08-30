package contributor

const (
	rendezvousDedicatedHostProfile       = "ardents-rendezvous-dedicated-host-v1"
	legacyRendezvousDedicatedHostProfile = "h4-5-rendezvous-alpha-v1"
)

func normalizeRendezvousDedicatedHostProfile(identity string) (string, bool) {
	switch identity {
	case rendezvousDedicatedHostProfile, legacyRendezvousDedicatedHostProfile:
		return rendezvousDedicatedHostProfile, true
	default:
		return "", false
	}
}
