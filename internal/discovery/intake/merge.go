package intake

import (
	discoveryfreshness "ardents/internal/discovery/freshness"
	discoveryrecord "ardents/internal/discovery/record"
)

func Upsert(entries []discoveryrecord.Entry, entry discoveryrecord.Entry) ([]discoveryrecord.Entry, ImportResult, error) {
	for i := range entries {
		if entries[i].Record.ID != entry.Record.ID {
			continue
		}
		if entries[i].Record.PublicKey != entry.Record.PublicKey {
			return entries, rejectedImportResult("rejected_conflict", "record id is already owned by a different public key"), nil
		}
		if discoveryfreshness.Score(entry.Record) < discoveryfreshness.Score(entries[i].Record) {
			return entries, rejectedImportResult("rejected_stale", "newer record is already present"), nil
		}
		entries[i] = entry
		return entries, importedResult("updated"), nil
	}
	for i := range entries {
		if entries[i].Record.Subject != entry.Record.Subject || entries[i].Record.Kind != entry.Record.Kind {
			continue
		}
		if entries[i].Record.PublicKey != entry.Record.PublicKey {
			return entries, rejectedImportResult("rejected_conflict", "record subject is already owned by a different public key"), nil
		}
		if discoveryfreshness.Score(entry.Record) < discoveryfreshness.Score(entries[i].Record) {
			return entries, rejectedImportResult("rejected_stale", "newer record is already present"), nil
		}
		entries[i] = entry
		return entries, importedResult("updated"), nil
	}
	return append(entries, entry), importedResult("imported"), nil
}
