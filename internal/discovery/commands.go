// Package discovery owns remote node and service knowledge workflows.
// It does not own local publication or carrier lifecycle.
package discovery

type ResolveRecordQuery struct {
	Subject string `json:"subject,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

type ResolveServiceQuery struct {
	Service string `json:"service,omitempty"`
}

type ListRecordsQuery struct{}

type ImportRecordCommand struct {
	Record CatalogRecordSnapshot `json:"record"`
}

type MutationGuard func(action string) error
type CommandEventSink func(topic string, fields map[string]any)

type CommandConfig struct {
	Guard     MutationGuard
	Emit      CommandEventSink
	OnChanged func()
}

type Commands struct {
	store *Service
	cfg   CommandConfig
}

func NewCommands(store *Service, cfg CommandConfig) *Commands {
	return &Commands{store: store, cfg: cfg}
}

func (c *Commands) ListRecords() ([]CatalogRecordSnapshot, error) {
	items := c.store.Entries()
	out := make([]CatalogRecordSnapshot, 0, len(items))
	for _, item := range items {
		out = append(out, RecordSnapshot(item))
	}
	return out, nil
}

func (c *Commands) ImportRecord(record CatalogRecordSnapshot) (RecordImportResult, error) {
	if c.cfg.Guard != nil {
		if err := c.cfg.Guard("discovery import"); err != nil {
			return RecordImportResult{}, err
		}
	}
	domainRecord := RecordFromSnapshot(record)
	result, err := c.store.Import(domainRecord, "")
	if err != nil {
		return RecordImportResult{}, err
	}
	if !result.Applied {
		return RecordImportResult{State: "rejected", Reason: result.Reason}, nil
	}
	if c.cfg.OnChanged != nil {
		c.cfg.OnChanged()
	}
	if c.cfg.Emit != nil {
		c.cfg.Emit("discovery.imported", map[string]any{"id": domainRecord.RecordID(), "subject": domainRecord.Subject(), "kind": domainRecord.Kind()})
	}
	return RecordImportResult{State: "completed", Reason: "record imported", Accepted: true}, nil
}
