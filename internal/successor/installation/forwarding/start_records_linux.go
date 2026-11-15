package forwarding

import "github.com/dianabuilds/ardents-network/internal/successor/installation/journal"

// Forwarding retains its own error vocabulary and independently owned journal.
func decodeStartRecords(raw []byte, number attemptNumber, journalPath, directory, binding, guard, socketRecord, archive, frame string, gid uint32) (map[string]startEntry, error) {
	return decodeStartRecordsFrames(raw, number, journalPath, directory, binding, guard, socketRecord, archive, frame, frame, gid)
}

func decodeStartRecordsFrames(raw []byte, number attemptNumber, journalPath, directory, binding, guard, socketRecord, archive, frame, acknowledgement string, gid uint32) (map[string]startEntry, error) {
	records, err := journal.DecodeStartRecords(raw, "forwarding", uint32(number), journalPath, directory, binding, guard, socketRecord, archive, frame, acknowledgement, gid)
	if err != nil {
		return nil, ErrBinding
	}
	return records, nil
}
