package records

import (
	"time"
)

type ImportResult struct {
	Outcome string
	Applied bool
	Reason  string
}

func Import(entries []Entry, record Record, source string, now time.Time) ([]Entry, ImportResult, error) {
	if source == "" {
		source = Imported
	}
	if err := Validate(record); err != nil {
		return entries, ImportResult{}, err
	}
	return Upsert(entries, Entry{
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
