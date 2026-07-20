package api

type ResolveRecordQuery struct {
	Subject string `json:"subject,omitempty"`
	Kind    string `json:"kind,omitempty"`
}

type ResolveServiceQuery struct {
	Service string `json:"service,omitempty"`
}

type ListRecordsQuery struct{}

type ImportRecordCommand struct {
	Record DiscoveryRecord `json:"record"`
}
