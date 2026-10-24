package infrastructure

import "errors"

var errSourceInitialPrefixBoundary = errors.New("source initial grammar: exact prefix boundary")

// This necessary syntactic bound is shared by recovery admission and ordinary
// recovered consumption. True supplies no native, Release or join authority.
// A complete suffix cannot authorize unsupported account/directory effects or
// an intent-only/incomplete fixed-unit birth.
func sourceInitialRecoveryPrefixSupported(records []sourceRecord) bool {
	count := len(records)
	return count >= 68 && count <= 115 && (count <= 110 || count >= 113) &&
		records[0].Phase == sourceRecoverableInitialPhase && records[count-1].Phase != "file-intent"
}

// A recovery classifier authenticates every available semantic record against
// independently reconstructed exact configuration. Only the new format with
// complete historical account birth may enter the native continuation owner.
// This grants no native custody, fresh Release, completed join or Start right.
func validateSourceRecoverableInitialPrefix(raw []byte, binding sourceBinding, hashes map[string]string, admitted string) error {
	if !sourceInitialSHA(admitted) {
		return ErrBinding
	}
	original, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil || len(original) < 41 {
		return ErrBinding
	}
	projected, err := projectSourceRecoverableInitialRecords(original, binding, hashes)
	if err != nil {
		return err
	}
	records, err := decodeSourceRecoveryPrefix(projected, 112)
	if err != nil {
		return err
	}
	_, err = walkSourceInitialRecords(records, binding, hashes, admitted)
	if err == errSourceInitialPrefixBoundary {
		return nil
	}
	return err
}
