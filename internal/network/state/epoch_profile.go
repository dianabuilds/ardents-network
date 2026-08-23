package state

import "errors"

const (
	roleProbeProfile        = "h3-role-probe-v1"
	interactiveRouteProfile = "ardents-interactive-route-v1"
)

func knownProfile(profile string) bool {
	return profile == roleProbeProfile || profile == interactiveRouteProfile
}

func matchProfile(expected, actual string) error {
	if expected == "" {
		expected = roleProbeProfile
	}
	if expected != actual {
		return errors.New("epoch profile does not match the configured consumer")
	}
	return nil
}

func requiredCapability(profile string) byte {
	if profile == interactiveRouteProfile {
		return 2
	}
	return 1
}
