package discovery

import (
	"fmt"

	db "ardents/internal/storage"

	discoveryrecord "ardents/internal/discovery/records"
	discoverytrust "ardents/internal/discovery/trust"
	identityprincipal "ardents/internal/identity/principal"
)

type Snapshot struct {
	SchemaVersion uint32                  `json:"schema_version"`
	Records       []discoveryrecord.Entry `json:"records"`
	State         string                  `json:"state"`
	Reason        string                  `json:"reason,omitempty"`
}

func (s *Snapshot) UnmarshalJSON(payload []byte) error {
	type snapshotWire struct {
		SchemaVersion *uint32                  `json:"schema_version"`
		Records       *[]discoveryrecord.Entry `json:"records"`
		State         *string                  `json:"state"`
		Reason        string                   `json:"reason,omitempty"`
	}
	var wire snapshotWire
	if err := db.DecodeJSONStrict(payload, &wire); err != nil {
		return err
	}
	if wire.SchemaVersion == nil || wire.Records == nil || wire.State == nil {
		return fmt.Errorf("discovery snapshot lacks required fields")
	}
	*s = Snapshot{SchemaVersion: *wire.SchemaVersion, Records: *wire.Records, State: *wire.State, Reason: wire.Reason}
	return nil
}

func PathInDir(dir string) string {
	return db.PathInDir(dir)
}

func LoadSnapshot(path string, out any) (bool, error) {
	return db.LoadJSONStrict(path, "discovery", "records", out)
}

func SaveSnapshot(path string, snapshot any) error {
	return db.SaveJSON(path, "discovery", "records", snapshot)
}

func CloneEntries(items []discoveryrecord.Entry) []discoveryrecord.Entry {
	out := make([]discoveryrecord.Entry, 0, len(items))
	for _, item := range items {
		item.Record = item.Record.Clone()
		out = append(out, item)
	}
	return out
}

func RecordSnapshot(entry discoveryrecord.Entry) CatalogRecordSnapshot {
	record := entry.Record
	out := CatalogRecordSnapshot{Version: record.Version, IssuedAt: record.IssuedAt, ExpiresAt: record.ExpiresAt, Signature: record.Signature, Source: entry.Source}
	if record.Node != nil {
		out.Node = &CatalogNodeFactsSnapshot{Principal: record.Node.Principal.String(), PublicKey: record.Node.PublicKey, Endpoints: append([]string(nil), record.Node.Endpoints...)}
	}
	if record.Service != nil {
		out.Service = &CatalogServiceFactsSnapshot{ID: string(record.Service.ID), Type: record.Service.Type, NodePrincipal: record.Service.NodePrincipal.String(), WorkloadID: string(record.Service.Workload), Mode: record.Service.Mode, PublicKey: record.Service.PublicKey, Endpoints: append([]string(nil), record.Service.Endpoints...)}
	}
	return out
}

func RecordFromSnapshot(in CatalogRecordSnapshot) Record {
	out := Record{Version: in.Version, IssuedAt: in.IssuedAt, ExpiresAt: in.ExpiresAt, Signature: in.Signature}
	if in.Node != nil {
		principal, _ := parseSnapshotPrincipal(in.Node.Principal)
		out.Node = &discoveryrecord.NodeFacts{Principal: principal, PublicKey: in.Node.PublicKey, Endpoints: append([]string(nil), in.Node.Endpoints...)}
	}
	if in.Service != nil {
		principal, _ := parseSnapshotPrincipal(in.Service.NodePrincipal)
		out.Service = &discoveryrecord.ServiceFacts{ID: discoveryrecord.ServiceID(in.Service.ID), Type: in.Service.Type, NodePrincipal: principal, Workload: discoveryrecord.WorkloadID(in.Service.WorkloadID), Mode: in.Service.Mode, PublicKey: in.Service.PublicKey, Endpoints: append([]string(nil), in.Service.Endpoints...)}
	}
	return out
}

func parseSnapshotPrincipal(raw string) (identityprincipal.ID, error) {
	principal, err := identityprincipal.Parse(raw)
	if err != nil {
		return identityprincipal.ID{}, fmt.Errorf("discovery Principal is invalid")
	}
	return principal, nil
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
