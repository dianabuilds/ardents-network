package journal

import (
	"bytes"
	"encoding/json"
	"path/filepath"
)

// The entire native transcript is required. A domain frame, result file or
// native-closed record found in a partial prefix is not committed retirement.
func DecodeRetirementRecords(raw []byte, purpose string, journal, directory, start, challenge, completed, result string, gid uint32) (map[string]InvocationRecord, error) {
	if purpose != "join" && purpose != "descriptor" && purpose != "introduction" && purpose != "forwarding" {
		return nil, ErrBinding
	}
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' || gid < 100 || gid > 999 {
		return nil, ErrBinding
	}
	for _, hash := range []string{start, challenge, completed, result} {
		if !validInvocationDigest(hash) {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 18 {
		return nil, ErrBinding
	}
	records := invocationRecords{lines: lines, schema: "ardents-" + purpose + "-retirement-journal-v1", owned: make(map[string]InvocationRecord)}
	next := records.next
	if next("stop", journal, start, 0600, 0, true) != nil || next("directory-intent", directory, "", 0, 0, false) != nil || next("directory-born", directory, "", 0700, 0, true) != nil || next("directory-ready", directory, "", 0750, gid, true) != nil {
		return nil, ErrBinding
	}
	file := func(name, hash string) error {
		path := filepath.Join(directory, name)
		if next("file-intent", path, hash, 0, 0, false) != nil || next("file-born", path, hash, 0600, 0, true) != nil || next("file-ready", path, hash, 0600, 0, true) != nil {
			return ErrBinding
		}
		return nil
	}
	pending := invocationDigest([]byte("ardents-" + purpose + "-retirement-pending-v1\n" + start + "\n"))
	socket := filepath.Join(directory, "stop-completion.socket")
	if file("retirement-pending", pending) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
		return nil, ErrBinding
	}
	var born InvocationRecord
	if records.position >= len(lines) || json.Unmarshal(lines[records.position], &born) != nil || born.Mode > 0777 {
		return nil, ErrBinding
	}
	if next("socket-born", socket, "", born.Mode, 0, true) != nil || next("socket-ready", socket, "", 0660, gid, true) != nil || next("interrupt-attempt", directory, challenge, 0, 0, false) != nil || next("domain-joined", directory, completed, 0, 0, false) != nil || next("native-joined", directory, result, 0, 0, false) != nil || next("native-closed", directory, result, 0, 0, false) != nil || file("retirement-result.json", result) != nil || next("retired", filepath.Join(directory, "retirement-result.json"), result, 0600, 0, true) != nil || records.position != len(lines) {
		return nil, ErrBinding
	}
	return records.owned, nil
}
