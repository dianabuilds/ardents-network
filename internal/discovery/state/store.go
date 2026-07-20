package state

import (
	db "ardents/internal/persistence"

	discoveryapi "ardents/internal/discovery/api"
	discoveryrecord "ardents/internal/discovery/record"
	discoverytrust "ardents/internal/discovery/trust"
)

type Snapshot struct {
	Records []discoveryrecord.Entry `json:"records"`
	State   string                  `json:"state"`
	Reason  string                  `json:"reason,omitempty"`
}

func PathInDir(dir string) string {
	return db.PathInDir(dir)
}

func LoadSnapshot(path string, out any) (bool, error) {
	return db.LoadJSON(path, "discovery", "records", out)
}

func SaveSnapshot(path string, snapshot any) error {
	return db.SaveJSON(path, "discovery", "records", snapshot)
}

func CloneEntries(items []discoveryrecord.Entry) []discoveryrecord.Entry {
	out := make([]discoveryrecord.Entry, 0, len(items))
	for _, item := range items {
		item.Record.Endpoints = append([]string(nil), item.Record.Endpoints...)
		out = append(out, item)
	}
	return out
}

func RecordSnapshot(entry discoveryrecord.Entry) discoveryapi.DiscoveryRecord {
	record := entry.Record
	return discoveryapi.DiscoveryRecord{
		ID:        record.ID,
		Kind:      record.Kind,
		Subject:   record.Subject,
		Node:      record.Node,
		Device:    record.Device,
		Owner:     record.Owner,
		Service:   record.Service,
		Mode:      record.Mode,
		PublicKey: record.PublicKey,
		Endpoints: append([]string(nil), record.Endpoints...),
		IssuedAt:  record.IssuedAt,
		ExpiresAt: record.ExpiresAt,
		Signature: record.Signature,
		Source:    entry.Source,
	}
}

func TrustSnapshot(state string, result discoverytrust.Result) discoveryapi.TrustSnapshot {
	return discoveryapi.TrustSnapshot{
		State:   state,
		Outcome: result.Outcome,
		Reason:  result.Reason,
		Valid:   result.Valid,
		Trusted: result.Trusted,
		Usable:  result.Usable,
	}
}
