package authority

import (
	"context"

	controlprojection "ardents/internal/control/projection"
	discovery "ardents/internal/discovery"
	discoveryapi "ardents/internal/discovery/api"
	discoverystate "ardents/internal/discovery/state"
	transport "ardents/internal/network/api"
)

func (c *Controller) ResolveRecordLocked(subject, kind string) (discoveryapi.DiscoveryResult, error) {
	entry, outcome, ok := c.disco.Resolve(subject, kind)
	if !ok {
		route := c.route.Preview(nil)
		return discoveryapi.DiscoveryResult{
			Outcome: outcome,
			Route:   controlprojection.Route(route),
		}, nil
	}
	if outcome != "found" {
		return c.rejectedDiscoveryResultLocked(entry, outcome), nil
	}
	trustResult := c.trust.Evaluate(entry.Record)
	if !c.transportAllowsRoutingLocked() {
		return c.resolveRecordWithoutRoutingLocked(entry, outcome, trustResult, "transport is not active"), nil
	}
	candidates := c.trans.BuildCandidates(entry.Record, trustResult.Trusted)
	candidates = c.filterRouteCandidatesLocked(subject, candidates)
	route := c.route.Preview(candidates)
	return discoveryapi.DiscoveryResult{
		Outcome:    outcome,
		Source:     entry.Source,
		Record:     controlprojection.Record(entry),
		Trust:      discoverystate.TrustSnapshot(discovery.TrustStateForResult(trustResult), trustResult),
		Candidates: controlprojection.Targets(candidates),
		Route:      controlprojection.Route(route),
	}, nil
}

func (c *Controller) ResolveServiceLocked(serviceType string) (discoveryapi.ServiceResult, error) {
	if err := c.SyncObservedWorkloadsLocked(context.Background()); err != nil {
		return discoveryapi.ServiceResult{}, err
	}
	entries := c.disco.FindService(serviceType)
	if len(entries) == 0 {
		route := c.route.Preview(nil)
		return discoveryapi.ServiceResult{
			Service: serviceType,
			Outcome: "not_found",
			Route:   controlprojection.Route(route),
		}, nil
	}
	if !c.transportAllowsRoutingLocked() {
		return c.resolveServiceWithoutRoutingLocked(serviceType, entries, "transport is not active"), nil
	}
	results := make([]discoveryapi.DiscoveryResult, 0, len(entries))
	allCandidates := make([]transport.Candidate, 0)
	for _, entry := range entries {
		trustResult := c.trust.Evaluate(entry.Record)
		candidates := c.trans.BuildCandidates(entry.Record, trustResult.Trusted)
		candidates = c.filterRouteCandidatesLocked(serviceType, candidates)
		allCandidates = append(allCandidates, candidates...)
		results = append(results, discoveryapi.DiscoveryResult{
			Outcome:    "found",
			Source:     entry.Source,
			Record:     controlprojection.Record(entry),
			Trust:      discoverystate.TrustSnapshot(discovery.TrustStateForResult(trustResult), trustResult),
			Candidates: controlprojection.Targets(candidates),
		})
	}
	route := c.route.Preview(allCandidates)
	return discoveryapi.ServiceResult{
		Service: serviceType,
		Outcome: route.Outcome,
		Matches: results,
		Route:   controlprojection.Route(route),
	}, nil
}

func (c *Controller) ListRecordsLocked() ([]discoveryapi.DiscoveryRecord, error) {
	items := c.disco.Entries()
	out := make([]discoveryapi.DiscoveryRecord, 0, len(items))
	for _, item := range items {
		out = append(out, controlprojection.Record(item))
	}
	return out, nil
}

func (c *Controller) ImportRecordLocked(record discoveryapi.DiscoveryRecord) (discoveryapi.RecordImportResult, error) {
	if err := c.requireAuthoritativeStateMutableLocked("discovery import"); err != nil {
		return discoveryapi.RecordImportResult{}, err
	}

	result, err := c.disco.Import(discovery.Record{
		ID:        record.ID,
		Kind:      record.Kind,
		Subject:   record.Subject,
		Node:      record.Node,
		Device:    record.Device,
		Owner:     record.Owner,
		Service:   record.Service,
		Mode:      record.Mode,
		PublicKey: record.PublicKey,
		Endpoints: cloneStrings(record.Endpoints),
		IssuedAt:  record.IssuedAt,
		ExpiresAt: record.ExpiresAt,
		Signature: record.Signature,
	}, record.Source)
	if err != nil {
		return discoveryapi.RecordImportResult{}, err
	}
	if !result.Applied {
		return discoveryapi.RecordImportResult{
			State:    "rejected",
			Reason:   result.Reason,
			Accepted: false,
		}, nil
	}
	c.SyncDiscoveryTrustDiagnosticsLocked()
	c.publish("discovery.imported", map[string]any{"id": record.ID, "subject": record.Subject, "kind": record.Kind})
	return discoveryapi.RecordImportResult{
		State:    "completed",
		Reason:   "record imported",
		Accepted: true,
	}, nil
}

func (c *Controller) rejectedDiscoveryResultLocked(entry discovery.Entry, outcome string) discoveryapi.DiscoveryResult {
	route := c.route.Preview(nil)
	trustResult := c.trust.Evaluate(entry.Record)
	return discoveryapi.DiscoveryResult{
		Outcome: outcome,
		Source:  entry.Source,
		Record:  controlprojection.Record(entry),
		Trust:   discoverystate.TrustSnapshot(discovery.TrustStateForResult(trustResult), trustResult),
		Route:   controlprojection.Route(route),
	}
}

func (c *Controller) resolveRecordWithoutRoutingLocked(entry discovery.Entry, outcome string, trustResult discovery.TrustResult, reason string) discoveryapi.DiscoveryResult {
	return discoveryapi.DiscoveryResult{
		Outcome: outcome,
		Source:  entry.Source,
		Record:  controlprojection.Record(entry),
		Trust:   discoverystate.TrustSnapshot(discovery.TrustStateForResult(trustResult), trustResult),
		Route:   c.unavailableRouteLocked(reason),
	}
}

func (c *Controller) resolveServiceWithoutRoutingLocked(serviceType string, entries []discovery.Entry, reason string) discoveryapi.ServiceResult {
	results := make([]discoveryapi.DiscoveryResult, 0, len(entries))
	for _, entry := range entries {
		trustResult := c.trust.Evaluate(entry.Record)
		results = append(results, discoveryapi.DiscoveryResult{
			Outcome: "found",
			Source:  entry.Source,
			Record:  controlprojection.Record(entry),
			Trust:   discoverystate.TrustSnapshot(discovery.TrustStateForResult(trustResult), trustResult),
		})
	}
	return discoveryapi.ServiceResult{
		Service: serviceType,
		Outcome: "not_usable",
		Matches: results,
		Route:   c.unavailableRouteLocked(reason),
	}
}
