package records

import (
	"errors"
	"time"
)

type ImportResult struct {
	Outcome string
	Applied bool
	Reason  string
}

// ImportVerified applies a record whose signature was verified by the owning
// discovery evaluator in the same call path.
func ImportVerified(entries []Entry, record Record, source string, now time.Time, evidence VerificationEvidence) ([]Entry, ImportResult, error) {
	if source == "" {
		source = Imported
	}
	if !ValidSource(source) {
		return entries, ImportResult{}, errors.New("record source is invalid")
	}
	if err := ValidateEvidence(record, evidence); err != nil {
		return entries, ImportResult{}, err
	}
	if now.Before(record.IssuedAt) {
		return entries, ImportResult{}, errors.New("record is not yet valid")
	}
	if !record.ExpiresAt.After(now) {
		return entries, ImportResult{}, errors.New("record expired")
	}
	return Upsert(entries, Entry{
		Record:   record,
		Source:   source,
		SeenAt:   now,
		Evidence: evidence,
	})
}

func importedResult(outcome string) ImportResult {
	return ImportResult{Outcome: outcome, Applied: true}
}

func rejectedImportResult(outcome, reason string) ImportResult {
	return ImportResult{Outcome: outcome, Reason: reason}
}
