package infrastructure

// Necessary producer/consumer classification only; no original-cut, native,
// predecessor retirement, State or Release authority comes from these bytes.
func sourceReplacementRecoveryPrefixSupported(records []sourceRecord) bool {
	count := len(records)
	return count >= 11 && count <= 55 && records[0].Phase == "initial" && records[count-1].Phase != "file-intent"
}

// Prefix validation returns no owned resources or continuation permission. The
// exact55 ordinary reader remains separate and requires a complete transcript.
func validateSourceReplacementPrefix(raw []byte, replacement sourceReplacementBinding, hashes map[string]string, admitted, previousUnit string) error {
	if !sourceInitialSHA(admitted) || !sourceInitialSHA(previousUnit) {
		return ErrBinding
	}
	records, err := decodeSourceRecoveryPrefix(raw, 55)
	if err != nil {
		return err
	}
	_, err = walkSourceReplacementRecords(records, replacement, hashes, admitted, previousUnit)
	if err == errSourceReplacementPrefixBoundary {
		return nil
	}
	return err
}
