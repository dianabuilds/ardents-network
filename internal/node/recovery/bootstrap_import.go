package recovery

import discovery "ardents/internal/discovery"

func ImportBootstrapEntries(
	localPrincipal string,
	entries []discovery.Entry,
	importRecord func(discovery.Record) (bool, error),
	degradeImport func(recordID, detail string),
	syncTrust func(),
) bool {
	hadImportErrors := false
	for _, entry := range entries {
		if entry.Record.Node == localPrincipal {
			continue
		}
		applied, err := importRecord(entry.Record)
		if err != nil {
			hadImportErrors = true
			degradeImport(entry.Record.ID, err.Error())
			continue
		}
		if applied {
			continue
		}
	}
	syncTrust()
	return hadImportErrors
}
