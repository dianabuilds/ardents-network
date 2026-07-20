package lifecycle

import transport "ardents/internal/network/api"

func (m *Manager) transportProfilePayloadLocked() map[string]any {
	snapshot := m.trans.ProfileSnapshot()
	return map[string]any{
		"profile":              string(snapshot.Profile),
		"mode":                 string(snapshot.Mode),
		"health":               string(snapshot.Health),
		"active_families":      transportFamilies(snapshot.ActiveFamilies),
		"suppressed_families":  transportFamilies(snapshot.SuppressedFamilies),
		"switch_reason":        string(snapshot.SwitchReason),
		"switch_automatic":     snapshot.SwitchAutomatic,
		"reduced_capabilities": append([]string(nil), snapshot.ReducedCapabilities...),
		"recovery_state":       string(snapshot.RecoveryState),
	}
}

func transportFamilies(items []transport.TransportFamily) []string {
	if len(items) == 0 {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, string(item))
	}
	return out
}
