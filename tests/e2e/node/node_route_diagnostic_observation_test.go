package state_test

import "time"

type routeDiagnosticObservation struct {
	at     time.Time
	reason string
}

func (process *nodeProcess) routeDiagnosticsAfter(since time.Time) ([]routeDiagnosticObservation, bool) {
	process.waitMu.Lock()
	defer process.waitMu.Unlock()
	var observed []routeDiagnosticObservation
	for _, event := range process.routeDiagnostics {
		if event.at.After(since) {
			observed = append(observed, event)
		}
	}
	return observed, process.latestDroppedRouteDiagnostic.After(since)
}
