package release

type qualificationState string
type buildLifecycleState string
type protocolLifecyclePhase string

const (
	qualificationQualified       qualificationState = "qualified"
	qualificationDevelopmentOnly qualificationState = "development-only"
	qualificationRevoked         qualificationState = "revoked"
	qualificationUnavailable     qualificationState = "unavailable"

	buildCurrent    buildLifecycleState = "current"
	buildSuperseded buildLifecycleState = "superseded"
	buildVulnerable buildLifecycleState = "vulnerable"
	buildRevoked    buildLifecycleState = "revoked"

	protocolAnnounced        protocolLifecyclePhase = "announced"
	protocolOverlapSupported protocolLifecyclePhase = "overlap-supported"
	protocolPreferred        protocolLifecyclePhase = "preferred"
	protocolRequired         protocolLifecyclePhase = "required"
	protocolRetired          protocolLifecyclePhase = "retired"
)

func parseQualification(raw string) (qualificationState, bool) {
	value := qualificationState(raw)
	switch value {
	case qualificationQualified, qualificationDevelopmentOnly, qualificationRevoked, qualificationUnavailable:
		return value, true
	default:
		return "", false
	}
}

func parseBuildState(raw string) (buildLifecycleState, bool) {
	value := buildLifecycleState(raw)
	switch value {
	case buildCurrent, buildSuperseded, buildVulnerable, buildRevoked:
		return value, true
	default:
		return "", false
	}
}

func parseProtocolPhase(raw string) (protocolLifecyclePhase, bool) {
	value := protocolLifecyclePhase(raw)
	switch value {
	case protocolAnnounced, protocolOverlapSupported, protocolPreferred, protocolRequired, protocolRetired:
		return value, true
	default:
		return "", false
	}
}
