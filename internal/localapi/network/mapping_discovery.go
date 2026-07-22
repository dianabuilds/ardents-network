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
	return &ardentsv1.DiscoveryRecord{
		Id: in.ID, Kind: in.Kind, Subject: in.Subject, Node: in.Node, Device: in.Device, Owner: in.Owner, Service: in.Service, Mode: in.Mode,
		PublicKey: in.PublicKey, Endpoints: append([]string(nil), in.Endpoints...), IssuedAt: rpc.Timestamp(in.IssuedAt), ExpiresAt: rpc.Timestamp(in.ExpiresAt), Signature: in.Signature, Source: in.Source,
	}
}

func fromDiscoveryRecord(in *ardentsv1.DiscoveryRecord) discoveryapi.CatalogRecordSnapshot {
	if in == nil {
		return discoveryapi.CatalogRecordSnapshot{}
	}
	return discoveryapi.CatalogRecordSnapshot{
		ID: in.GetId(), Kind: in.GetKind(), Subject: in.GetSubject(), Node: in.GetNode(), Device: in.GetDevice(), Owner: in.GetOwner(), Service: in.GetService(), Mode: in.GetMode(),
		PublicKey: in.GetPublicKey(), Endpoints: append([]string(nil), in.GetEndpoints()...), IssuedAt: rpc.Time(in.GetIssuedAt()), ExpiresAt: rpc.Time(in.GetExpiresAt()), Signature: in.GetSignature(),
	}
}

func toDiscoveryResult(in discoveryapi.ResolutionResult) *ardentsv1.DiscoveryResult {
	out := &ardentsv1.DiscoveryResult{
		Outcome: in.Outcome,
		Source:  in.Source,
		Record:  toDiscoveryRecord(in.Record),
		Trust:   toDiscoveryTrustSnapshot(in.Trust),
		Route:   toRouteSnapshot(in.Route),
	}
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
	return &ardentsv1.TransportTarget{
		Subject: in.Subject, Service: in.Service, Endpoint: in.Endpoint, Scheme: in.Scheme, Mode: in.Mode,
		Trusted: in.Trusted, Usable: in.Usable, Cost: int32(in.Cost), Privacy: int32(in.Privacy), Reliability: int32(in.Reliability),
	}
}

func toRouteSnapshot(in discoveryapi.RouteSnapshot) *ardentsv1.RouteSnapshot {
	out := &ardentsv1.RouteSnapshot{Outcome: in.Outcome, Reason: in.Reason, Candidates: int32(in.Candidates), Usable: int32(in.Usable)}
	if in.Selected != nil {
		out.Selected = toTransportTarget(*in.Selected)
	}
	return out
}
