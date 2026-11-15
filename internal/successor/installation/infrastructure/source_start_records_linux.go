package infrastructure

import "github.com/dianabuilds/ardents-network/internal/successor/installation/journal"

// Source retains the role-specific error vocabulary and original custody.
func decodeSourceStartRecords(raw []byte, number sourceAttemptNumber, journalPath, directory, binding, guard, socketRecord, archive, frame string, gid uint32) (map[string]sourceStartEntry, error) {
	records, err := journal.DecodeSourceStartRecords(raw, uint32(number), journalPath, directory, binding, guard, socketRecord, archive, frame, gid)
	if err != nil {
		return nil, ErrBinding
	}
	return records, nil
}
