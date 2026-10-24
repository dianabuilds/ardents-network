package infrastructure

import (
	"bytes"
	"encoding/json"
)

// Legacy112 and recoverable115 are separate complete grammars. A normalized
// detached view is passed to the unchanged legacy validator only after the new
// header and the exact original pending birth have been checked. Nothing is
// rewritten on disk and no partial prefix enters the ordinary reader.
func decodeSourceInitialBirthRecords(raw []byte, binding sourceBinding, hashes map[string]string, admitted string) (map[string]sourceRecord, error) {
	end := bytes.IndexByte(raw, '\n')
	if end < 0 || end+1 > 32<<10 || len(raw) > 4<<20 {
		return nil, ErrBinding
	}
	var first sourceRecord
	if json.Unmarshal(raw[:end], &first) != nil {
		return nil, ErrBinding
	}
	if first.Phase == "initial" {
		return decodeSourceInitialRecords(raw, binding, hashes, admitted)
	}
	if first.Phase != sourceRecoverableInitialPhase {
		return nil, ErrBinding
	}
	records, err := decodeSourceRecoveryPrefix(raw, sourceRecoverableInitialRecords)
	if err != nil || len(records) != sourceRecoverableInitialRecords {
		return nil, ErrBinding
	}
	projected, err := projectSourceRecoverableInitialRecords(records, binding, hashes)
	if err != nil {
		return nil, err
	}
	return decodeSourceInitialRecords(projected, binding, hashes, admitted)
}

func projectSourceRecoverableInitialRecords(records []sourceRecord, binding sourceBinding, hashes map[string]string) ([]byte, error) {
	if len(records) < 4 || len(records) > sourceRecoverableInitialRecords || records[0].Phase != sourceRecoverableInitialPhase {
		return nil, ErrBinding
	}
	path := sourceInitialPendingPath(binding.Request.InstallationRoot)
	sha := digest(sourceInitialPending(hashes[binding.Request.InstallationRoot+"/binding.json"]))
	intent := sourceRecord{Sequence: 1, Phase: "file-intent", Path: path, SHA256: sha, Mode: 0600}
	born := intent
	born.Sequence, born.Phase = 2, "file-born"
	born.Device, born.Inode = records[2].Device, records[2].Inode
	ready := born
	ready.Sequence, ready.Phase = 3, "file-ready"
	if records[1] != intent || born.Device == 0 || born.Inode == 0 || records[2] != born || records[3] != ready {
		return nil, ErrBinding
	}
	projected := make([]byte, 0)
	for i, record := range records {
		if i >= 1 && i <= 3 {
			continue
		}
		if i == 0 {
			record.Phase = "initial"
		} else {
			record.Sequence -= 3
		}
		line, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		projected = append(projected, line...)
		projected = append(projected, '\n')
	}
	// The pending file must be absent at ordinary publication, like replacement's
	// transient pending file. Its historical born identity is not a live resource.
	return projected, nil
}
