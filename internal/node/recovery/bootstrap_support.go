package recovery

import (
	"strings"

	"ardents/internal/diagnostics"
	transport "ardents/internal/network/api"
)

func NetworkBootstrapSources(items []string) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		trimmed := strings.TrimSpace(item)
		if strings.HasPrefix(trimmed, "/") {
			out = append(out, trimmed)
		}
	}
	return out
}

func RecordBootstrapDial(rec *diagnostics.Recorder, nodeName string, report transport.BootstrapDialReport) {
	if rec == nil {
		return
	}
	if report.Success {
		rec.RecordEvent("transport", "bootstrap_dial_succeeded", nodeName, "bootstrap peer dial succeeded", "", map[string]any{
			"peer": report.Peer,
		})
		return
	}
	rec.RecordEvent("transport", "bootstrap_dial_failed", nodeName, "bootstrap peer dial failed", "transport.bootstrap.dial_failed", map[string]any{
		"peer":   report.Peer,
		"detail": report.Detail,
	})
}
