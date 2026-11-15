package journal

import (
	"bytes"
	"encoding/json"
	"path/filepath"
)

// DecodeSourceRetirementRecords validates the complete Source-specific
// twelve-phase transcript. Detached records confer no Source authority.
func DecodeSourceRetirementRecords(raw []byte, directory, start, challenge, completed, result string, gid uint32) (map[string]InvocationRecord, error) {
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' || gid < 100 || gid > 999 {
		return nil, ErrBinding
	}
	for _, hash := range []string{start, challenge, completed, result} {
		if !validInvocationDigest(hash) {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 12 {
		return nil, ErrBinding
	}
	records := invocationRecords{lines: lines, schema: "ardents-source-retirement-record-v1", owned: make(map[string]InvocationRecord)}
	next := records.next

	journal := filepath.Join(directory, "source-retirement-journal")
	socket := filepath.Join(directory, "stop-completion.socket")
	file := filepath.Join(directory, "source-retirement.json")
	if next("stop", journal, start, 0600, 0, true) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
		return nil, ErrBinding
	}
	var birth InvocationRecord
	if json.Unmarshal(lines[records.position], &birth) != nil || birth.Mode > 0777 {
		return nil, ErrBinding
	}
	if next("socket-born", socket, "", birth.Mode, 0, true) != nil || next("socket-ready", socket, "", 0660, gid, true) != nil ||
		next("interrupt-attempt", directory, challenge, 0, 0, false) != nil ||
		next("domain-joined", directory, completed, 0, 0, false) != nil ||
		next("native-joined", directory, result, 0, 0, false) != nil ||
		next("native-closed", directory, result, 0, 0, false) != nil ||
		next("file-intent", file, result, 0, 0, false) != nil ||
		next("file-born", file, result, 0600, 0, true) != nil ||
		next("file-ready", file, result, 0600, 0, true) != nil ||
		next("retired", file, result, 0600, 0, true) != nil || records.position != len(lines) {
		return nil, ErrBinding
	}
	return records.owned, nil
}
