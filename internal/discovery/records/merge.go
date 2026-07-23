package records

func Upsert(entries []Entry, entry Entry) ([]Entry, ImportResult, error) {
	updated := make([]Entry, len(entries))
	copy(updated, entries)
	entries = updated
	for i := range entries {
		if entries[i].Record.RecordID() != entry.Record.RecordID() {
			continue
		}
		if entries[i].Record.PublicKeyText() != entry.Record.PublicKeyText() {
			return entries, rejectedImportResult("rejected_conflict", "record id is already owned by a different public key"), nil
		}
		if Score(entry.Record) < Score(entries[i].Record) {
			return entries, rejectedImportResult("rejected_stale", "newer record is already present"), nil
		}
		entries[i] = entry
		return entries, importedResult("updated"), nil
	}
	for i := range entries {
		if entries[i].Record.Subject() != entry.Record.Subject() || entries[i].Record.Kind() != entry.Record.Kind() {
			continue
		}
		if entries[i].Record.PublicKeyText() != entry.Record.PublicKeyText() {
			return entries, rejectedImportResult("rejected_conflict", "record subject is already owned by a different public key"), nil
		}
		if Score(entry.Record) < Score(entries[i].Record) {
			return entries, rejectedImportResult("rejected_stale", "newer record is already present"), nil
		}
		entries[i] = entry
		return entries, importedResult("updated"), nil
	}
	return append(entries, entry), importedResult("imported"), nil
}
