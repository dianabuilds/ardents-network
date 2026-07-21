package discovery

import "time"

func ProjectStatus(state, reason string, entries []Entry, now time.Time, evaluate func(Record) TrustResult) StatusSnapshot {
	status := StatusSnapshot{State: state, Reason: reason}
	for _, item := range entries {
		if item.Source == "local" {
			status.LocalRecords++
			if item.SeenAt.After(status.LastPublishAt) {
				status.LastPublishAt = item.SeenAt
			}
		} else {
			status.RemoteRecords++
			if item.SeenAt.After(status.LastRefreshAt) {
				status.LastRefreshAt = item.SeenAt
			}
		}
		if !item.Record.ExpiresAt.IsZero() && now.After(item.Record.ExpiresAt) {
			status.StaleRecords++
			status.RejectedRecords++
			continue
		}
		trust := evaluate(item.Record)
		if trust.Trusted && trust.Usable {
			status.TrustedRecords++
			continue
		}
		if !trust.Usable || !trust.Valid || trust.Outcome == "expired" {
			status.RejectedRecords++
		}
	}
	return status
}

type Reachability func(Record, bool) (state, reason string)

func ProjectPeers(entries []Entry, localID string, reachability Reachability, evaluate func(Record) TrustResult) []PeerSnapshot {
	peers := make([]PeerSnapshot, 0, len(entries))
	for _, item := range entries {
		if item.Record.Kind != "node" || item.Record.Subject == localID {
			continue
		}
		trust := evaluate(item.Record)
		reachabilityState, reason := reachability(item.Record, trust.Trusted)
		state := "ready"
		if !trust.Usable || reachabilityState != "reachable" {
			state = "degraded"
		}
		if !trust.Valid {
			state = "failed"
		}
		if reason == "" {
			reason = trust.Reason
		}
		peers = append(peers, PeerSnapshot{
			NodeID: item.Record.Node, DeviceID: item.Record.Device,
			Addresses: append([]string(nil), item.Record.Endpoints...),
			Trust:     ProjectTrust(TrustStateForResult(trust), trust), Reachability: reachabilityState,
			Source: item.Source, LastSeenAt: item.SeenAt, State: state, Reason: reason,
		})
	}
	return peers
}
