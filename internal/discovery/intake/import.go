package intake

import (
	"time"

	discoveryrecord "ardents/internal/discovery/record"
	discoverysource "ardents/internal/discovery/source"
)

type ImportResult struct {
	Outcome string
	Applied bool
	Reason  string
}

func Import(entries []discoveryrecord.Entry, record discoveryrecord.Record, source string, now time.Time) ([]discoveryrecord.Entry, ImportResult, error) {
	if source == "" {
		source = discoverysource.Imported
	}
	if err := discoveryrecord.Validate(record); err != nil {
		return entries, ImportResult{}, err
	}
	return Upsert(entries, discoveryrecord.Entry{
		Record: record,
		Source: source,
		SeenAt: now,
	})
}

func importedResult(outcome string) ImportResult {
	return ImportResult{Outcome: outcome, Applied: true}
}

func rejectedImportResult(outcome, reason string) ImportResult {
	return ImportResult{Outcome: outcome, Reason: reason}
}
