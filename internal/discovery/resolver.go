package discovery

import (
	"context"

	"ardents/internal/network"
	"ardents/internal/network/routing"
)

type RoutePolicy interface {
	AllowRouteUse(network.Candidate) error
}

type ResolverConfig struct {
	Store                *Service
	Trust                *TrustEvaluator
	Network              network.Service
	Routes               *routing.State
	Policy               RoutePolicy
	BeforeServiceResolve func(context.Context) error
	PolicyDenied         func(resource, action string, err error)
}

type Resolver struct{ cfg ResolverConfig }

func NewResolver(cfg ResolverConfig) *Resolver { return &Resolver{cfg: cfg} }

func (r *Resolver) ResolveRecord(subject, kind string) (ResolutionResult, error) {
	entry, outcome, ok := r.cfg.Store.Resolve(subject, kind)
	if !ok {
		return ResolutionResult{Outcome: outcome, Route: ProjectRoute(r.cfg.Routes.Preview(nil))}, nil
	}
	if outcome != "found" {
		return r.rejected(entry, outcome), nil
	}
	trust := r.cfg.Trust.Evaluate(entry.Record)
	if r.cfg.Network.State() != "ready" {
		return r.withoutRoute(entry, outcome, trust, "transport is not active"), nil
	}
	candidates := r.allowed(subject, r.cfg.Network.BuildCandidates(routeRecord(entry.Record), trust.Trusted))
	return ResolutionResult{
		Outcome: outcome, Source: entry.Source, Record: RecordSnapshot(entry),
		Trust:      ProjectTrust(TrustStateForResult(trust), trust),
		Candidates: ProjectTargets(candidates), Route: ProjectRoute(r.cfg.Routes.Preview(candidates)),
	}, nil
}

func (r *Resolver) ResolveService(serviceType string) (ServiceResult, error) {
	if r.cfg.BeforeServiceResolve != nil {
		if err := r.cfg.BeforeServiceResolve(context.Background()); err != nil {
			return ServiceResult{}, err
		}
	}
	entries := r.cfg.Store.FindService(serviceType)
	if len(entries) == 0 {
		return ServiceResult{Service: serviceType, Outcome: "not_found", Route: ProjectRoute(r.cfg.Routes.Preview(nil))}, nil
	}
	if r.cfg.Network.State() != "ready" {
		return r.serviceWithoutRoute(serviceType, entries, "transport is not active"), nil
	}
	results := make([]ResolutionResult, 0, len(entries))
	all := make([]network.Candidate, 0)
	for _, entry := range entries {
		trust := r.cfg.Trust.Evaluate(entry.Record)
		candidates := r.allowed(serviceType, r.cfg.Network.BuildCandidates(routeRecord(entry.Record), trust.Trusted))
		all = append(all, candidates...)
		results = append(results, ResolutionResult{
			Outcome: "found", Source: entry.Source, Record: RecordSnapshot(entry),
			Trust: ProjectTrust(TrustStateForResult(trust), trust), Candidates: ProjectTargets(candidates),
		})
	}
	route := r.cfg.Routes.Preview(all)
	return ServiceResult{Service: serviceType, Outcome: route.Outcome, Matches: results, Route: ProjectRoute(route)}, nil
}

func (r *Resolver) allowed(resource string, candidates []network.Candidate) []network.Candidate {
	if len(candidates) == 0 {
		return nil
	}
	out := make([]network.Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		if r.cfg.Policy != nil {
			if err := r.cfg.Policy.AllowRouteUse(candidate); err != nil {
				if r.cfg.PolicyDenied != nil {
					r.cfg.PolicyDenied(resource, "route.use", err)
				}
				continue
			}
		}
		out = append(out, candidate)
	}
	return out
}

func (r *Resolver) rejected(entry Entry, outcome string) ResolutionResult {
	trust := r.cfg.Trust.Evaluate(entry.Record)
	return ResolutionResult{Outcome: outcome, Source: entry.Source, Record: RecordSnapshot(entry),
		Trust: ProjectTrust(TrustStateForResult(trust), trust), Route: ProjectRoute(r.cfg.Routes.Preview(nil))}
}

func (r *Resolver) withoutRoute(entry Entry, outcome string, trust TrustResult, reason string) ResolutionResult {
	return ResolutionResult{Outcome: outcome, Source: entry.Source, Record: RecordSnapshot(entry),
		Trust: ProjectTrust(TrustStateForResult(trust), trust), Route: ProjectRoute(r.cfg.Routes.PreviewUnavailable(reason))}
}

func (r *Resolver) serviceWithoutRoute(service string, entries []Entry, reason string) ServiceResult {
	results := make([]ResolutionResult, 0, len(entries))
	for _, entry := range entries {
		trust := r.cfg.Trust.Evaluate(entry.Record)
		results = append(results, ResolutionResult{Outcome: "found", Source: entry.Source,
			Record: RecordSnapshot(entry), Trust: ProjectTrust(TrustStateForResult(trust), trust)})
	}
	return ServiceResult{Service: service, Outcome: "not_usable", Matches: results,
		Route: ProjectRoute(r.cfg.Routes.PreviewUnavailable(reason))}
}

func ProjectTargets(items []network.Candidate) []TransportTarget {
	if len(items) == 0 {
		return nil
	}
	out := make([]TransportTarget, 0, len(items))
	for _, item := range items {
		out = append(out, TransportTarget{Subject: item.Subject, Service: item.Service,
			Endpoint: item.Endpoint, Scheme: item.Scheme, Mode: item.Mode, Trusted: item.Trusted,
			Usable: item.Usable, Cost: item.Cost, Privacy: item.Privacy, Reliability: item.Reliability})
	}
	return out
}

func ProjectRoute(in routing.Snapshot) RouteSnapshot {
	out := RouteSnapshot{Outcome: in.Outcome, Reason: in.Reason, Candidates: in.Candidates, Usable: in.Usable}
	if in.Selected != nil {
		selected := ProjectTargets([]network.Candidate{*in.Selected})
		if len(selected) != 0 {
			out.Selected = &selected[0]
		}
	}
	return out
}

func routeRecord(record Record) network.RouteRecord {
	return network.RouteRecord{Subject: record.Subject, Service: record.Service, Mode: record.Mode, Endpoints: append([]string(nil), record.Endpoints...)}
}
