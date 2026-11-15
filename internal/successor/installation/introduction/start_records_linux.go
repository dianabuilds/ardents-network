package introduction

import "github.com/dianabuilds/ardents-network/internal/successor/installation/journal"

// Introduction retains its own error vocabulary and independently owned journal.
func decodeStartRecordsFrames(raw []byte, journalPath, directory, binding, guard, socketRecord, archive, frame, acknowledgement string, gid uint32) (map[string]startEntry, error) {
	records, err := journal.DecodeStartRecords(raw, "introduction", 0, journalPath, directory, binding, guard, socketRecord, archive, frame, acknowledgement, gid)
	if err != nil {
		return nil, ErrBinding
	}
	return records, nil
}
