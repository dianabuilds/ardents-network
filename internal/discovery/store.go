package discovery

import (
	db "ardents/internal/storage"

	discoveryrecord "ardents/internal/discovery/records"
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

func RecordSnapshot(entry discoveryrecord.Entry) CatalogRecordSnapshot {
	record := entry.Record
	return CatalogRecordSnapshot{
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

func RecordFromSnapshot(in CatalogRecordSnapshot) Record {
	return Record{
		ID: in.ID, Kind: in.Kind, Subject: in.Subject, Node: in.Node,
		Device: in.Device, Owner: in.Owner, Service: in.Service, Mode: in.Mode,
		PublicKey: in.PublicKey, Endpoints: append([]string(nil), in.Endpoints...),
		IssuedAt: in.IssuedAt, ExpiresAt: in.ExpiresAt, Signature: in.Signature,
	}
}

func ProjectTrust(state string, result discoverytrust.Result) TrustSnapshot {
	return TrustSnapshot{
		State:   state,
		Outcome: result.Outcome,
		Reason:  result.Reason,
		Valid:   result.Valid,
		Trusted: result.Trusted,
		Usable:  result.Usable,
	}
}
