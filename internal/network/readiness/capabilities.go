package readiness

func activeMessagingCapabilities(state ServiceState) []string {
	if state.State != "ready" && state.State != "degraded" {
		return nil
	}
	if state.NodeProfile != NodeProfileConstrainedClient {
		if state.ActiveMode == ModeRestrictedDefense {
			return []string{"relay"}
		}
		return []string{"relay", "store", "filter_service", "lightpush_service"}
	}
	var active []string
	if state.FilterPeerCount > 0 {
		active = append(active, "filter_client")
	}
	if state.LightpushPeerCount > 0 {
		active = append(active, "lightpush_client")
	}
	if state.StorePeerCount > 0 {
		active = append(active, "store_client")
	}
	return active
}
