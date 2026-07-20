package projection

import discovery "ardents/internal/discovery"

func (r *Reader) observedTrustSnapshotLocked() (discovery.TrustResult, string) {
	localID := r.ident.NodeSummary().Principal
	var (
		localResult   discovery.TrustResult
		localFound    bool
		degradedTrust discovery.TrustResult
		degradedFound bool
	)
	for _, entry := range r.disco.Entries() {
		result := r.trust.Evaluate(entry.Record)
		if entry.Source == "local" && entry.Record.Kind == "node" && entry.Record.Subject == localID {
			localResult = result
			localFound = true
		}
		if result.Usable || trustResultIsAdvisory(result) {
			continue
		}
		if !degradedFound || trustResultSeverity(result) > trustResultSeverity(degradedTrust) {
			degradedTrust = result
			degradedFound = true
		}
	}
	switch {
	case degradedFound:
		return degradedTrust, discovery.TrustStateForResult(degradedTrust)
	case localFound:
		return localResult, discovery.TrustStateForResult(localResult)
	default:
		last := r.trust.Last()
		return last, r.trust.State()
	}
}

func trustResultIsAdvisory(result discovery.TrustResult) bool {
	return result.Valid && !result.Trusted
}

func trustResultSeverity(result discovery.TrustResult) int {
	switch {
	case result.Outcome == "expired":
		return 2
	case !result.Valid:
		return 2
	default:
		return 1
	}
}
