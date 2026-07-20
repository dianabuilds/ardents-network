package connectrpc

import (
	discoveryapi "ardents/internal/discovery/api"
	nodeapi "ardents/internal/node/api"
	ardentsv1 "ardents/proto/ardents/v1"
)

func toDiscoveryStatusSnapshot(in nodeapi.DiscoveryStatusSnapshot) *ardentsv1.DiscoveryStatusSnapshot {
	return &ardentsv1.DiscoveryStatusSnapshot{
		State:           in.State,
		Reason:          in.Reason,
		LocalRecords:    int32(in.LocalRecords),
		RemoteRecords:   int32(in.RemoteRecords),
		TrustedRecords:  int32(in.TrustedRecords),
		RejectedRecords: int32(in.RejectedRecords),
		StaleRecords:    int32(in.StaleRecords),
		LastPublishAt:   ts(in.LastPublishAt),
		LastRefreshAt:   ts(in.LastRefreshAt),
	}
}

func toLocalPresenceSnapshot(in nodeapi.LocalPresenceSnapshot) *ardentsv1.LocalPresenceSnapshot {
	return &ardentsv1.LocalPresenceSnapshot{
		Published:              in.Published,
		State:                  in.State,
		Reason:                 in.Reason,
		RecordId:               in.RecordID,
		PublishedAt:            ts(in.PublishedAt),
		ExpiresAt:              ts(in.ExpiresAt),
		OperatorActionRequired: in.OperatorActionRequired,
	}
}

func toRouteCandidateSnapshots(items []discoveryapi.RouteCandidateSnapshot) []*ardentsv1.RouteCandidateSnapshot {
	out := make([]*ardentsv1.RouteCandidateSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, &ardentsv1.RouteCandidateSnapshot{
			Subject:     item.Subject,
			Service:     item.Service,
			Endpoint:    item.Endpoint,
			Scheme:      item.Scheme,
			Mode:        item.Mode,
			Trusted:     item.Trusted,
			Usable:      item.Usable,
			Cost:        int32(item.Cost),
			Privacy:     int32(item.Privacy),
			Reliability: int32(item.Reliability),
			State:       item.State,
			Reason:      item.Reason,
		})
	}
	return out
}
