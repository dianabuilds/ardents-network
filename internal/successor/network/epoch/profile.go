package epoch

import "errors"

const (
	roleProbeProfile        = "h3-role-probe-v1"
	interactiveRouteProfile = "ardents-interactive-route-v2"
	closedRouteProfile      = "ardents-route-v3"
)

const (
	ProfileRoleProbe        = roleProbeProfile
	ProfileInteractiveRoute = interactiveRouteProfile
	ProfileClosedRoute      = closedRouteProfile
)

func KnownProfile(profile string) bool {
	return profile == roleProbeProfile || profile == interactiveRouteProfile || profile == closedRouteProfile
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
