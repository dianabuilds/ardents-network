package network

import (
	discoveryapi "ardents/internal/discovery"

	ardentsv1 "ardents/internal/localapi/protocol"
	"ardents/internal/localapi/rpc"
)

func toDiscoveryTrustSnapshot(in discoveryapi.TrustSnapshot) *ardentsv1.TrustSnapshot {
	return &ardentsv1.TrustSnapshot{State: in.State, Outcome: in.Outcome, Reason: in.Reason, Valid: in.Valid, Trusted: in.Trusted, Usable: in.Usable}
}

func toDiscoveryRecord(in discoveryapi.CatalogRecordSnapshot) *ardentsv1.DiscoveryRecord {
	out := &ardentsv1.DiscoveryRecord{
		Version: in.Version, IssuedAtV1: rpc.Timestamp(in.IssuedAt), ExpiresAtV1: rpc.Timestamp(in.ExpiresAt),
		SignatureV1: in.Signature, SourceV1: in.Source,
	}
	if in.Node != nil {
		out.Facts = &ardentsv1.DiscoveryRecord_NodeFacts{NodeFacts: &ardentsv1.NodeDiscoveryFacts{
			Principal: in.Node.Principal, PublicKey: in.Node.PublicKey, Endpoints: append([]string(nil), in.Node.Endpoints...),
		}}
	}
	if in.Service != nil {
		out.Facts = &ardentsv1.DiscoveryRecord_ServiceFacts{ServiceFacts: &ardentsv1.ServiceDiscoveryFacts{
			ServiceId: in.Service.ID, ServiceType: in.Service.Type, NodePrincipal: in.Service.NodePrincipal,
			WorkloadId: in.Service.WorkloadID, Mode: in.Service.Mode, PublicKey: in.Service.PublicKey,
			Endpoints: append([]string(nil), in.Service.Endpoints...),
		}}
	}
	return out
}

func fromDiscoveryRecord(in *ardentsv1.DiscoveryRecord) discoveryapi.CatalogRecordSnapshot {
	if in == nil {
		return discoveryapi.CatalogRecordSnapshot{}
	}
	out := discoveryapi.CatalogRecordSnapshot{
		Version: in.GetVersion(), IssuedAt: rpc.Time(in.GetIssuedAtV1()), ExpiresAt: rpc.Time(in.GetExpiresAtV1()),
		Signature: in.GetSignatureV1(),
	}
	if facts := in.GetNodeFacts(); facts != nil {
		out.Node = &discoveryapi.CatalogNodeFactsSnapshot{Principal: facts.GetPrincipal(), PublicKey: facts.GetPublicKey(), Endpoints: append([]string(nil), facts.GetEndpoints()...)}
	}
	if facts := in.GetServiceFacts(); facts != nil {
		out.Service = &discoveryapi.CatalogServiceFactsSnapshot{
			ID: facts.GetServiceId(), Type: facts.GetServiceType(), NodePrincipal: facts.GetNodePrincipal(), WorkloadID: facts.GetWorkloadId(),
			Mode: facts.GetMode(), PublicKey: facts.GetPublicKey(), Endpoints: append([]string(nil), facts.GetEndpoints()...),
		}
	}
	return out
}

func toDiscoveryResult(in discoveryapi.ResolutionResult) *ardentsv1.DiscoveryResult {
	out := &ardentsv1.DiscoveryResult{Outcome: in.Outcome, Source: in.Source, Record: toDiscoveryRecord(in.Record), Trust: toDiscoveryTrustSnapshot(in.Trust), Route: toRouteSnapshot(in.Route)}
	for _, item := range in.Candidates {
		out.Candidates = append(out.Candidates, toTransportTarget(item))
	}
	return out
}

func toServiceResult(in discoveryapi.ServiceResult) *ardentsv1.ServiceResult {
	out := &ardentsv1.ServiceResult{Service: in.Service, Outcome: in.Outcome, Route: toRouteSnapshot(in.Route)}
	for _, item := range in.Matches {
		out.Matches = append(out.Matches, toDiscoveryResult(item))
	}
	return out
}

func toTransportTarget(in discoveryapi.TransportTarget) *ardentsv1.TransportTarget {
	return &ardentsv1.TransportTarget{Subject: in.Subject, Service: in.Service, Endpoint: in.Endpoint, Scheme: in.Scheme, Mode: in.Mode, Trusted: in.Trusted, Usable: in.Usable, Cost: int32(in.Cost), Privacy: int32(in.Privacy), Reliability: int32(in.Reliability)}
}

func toRouteSnapshot(in discoveryapi.RouteSnapshot) *ardentsv1.RouteSnapshot {
	out := &ardentsv1.RouteSnapshot{Outcome: in.Outcome, Reason: in.Reason, Candidates: int32(in.Candidates), Usable: int32(in.Usable)}
	if in.Selected != nil {
		out.Selected = toTransportTarget(*in.Selected)
	}
	return out
}
