package api

type Service interface {
	ListRecords() ([]DiscoveryRecord, error)
	ResolveRecord(string, string) (DiscoveryResult, error)
	ImportRecord(DiscoveryRecord) (RecordImportResult, error)
	ResolveService(string) (ServiceResult, error)
}
