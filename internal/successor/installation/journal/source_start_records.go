package journal

import (
	"bytes"
	"encoding/json"
	"path/filepath"
)

// DecodeSourceStartRecords validates the complete Source-specific start transcript.
// It retains no native custody and grants no Start, ACK or recovery authority.
func DecodeSourceStartRecords(raw []byte, number uint32, journal, directory, binding, guard, socketRecord, archive, frame string, gid uint32) (map[string]InvocationRecord, error) {
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' || gid < 100 || gid > 999 || number >= 64 {
		return nil, ErrBinding
	}
	for _, hash := range []string{binding, guard, socketRecord, archive, frame} {
		if !validInvocationDigest(hash) {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 22 {
		return nil, ErrBinding
	}
	schema, firstPhase := "ardents-source-first-start-record-v1", "first-start"
	if number != 0 {
		schema, firstPhase = "ardents-source-invocation-record-v1", "invocation"
	}
	records := invocationRecords{lines: lines, schema: schema, owned: make(map[string]InvocationRecord)}
	next := records.next

	if next(firstPhase, journal, binding, 0600, 0, true) != nil ||
		next("directory-intent", directory, "", 0, 0, false) != nil ||
		next("directory-born", directory, "", 0700, 0, true) != nil ||
		next("directory-ready", directory, "", 0750, gid, true) != nil {
		return nil, ErrBinding
	}
	file := func(name, hash string) error {
		path := filepath.Join(directory, name)
		if next("file-intent", path, hash, 0, 0, false) != nil ||
			next("file-born", path, hash, 0600, 0, true) != nil ||
			next("file-ready", path, hash, 0600, 0, true) != nil {
			return ErrBinding
		}
		return nil
	}
	socket := filepath.Join(directory, "start-completion.socket")
	if file("start-guard.json", guard) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
		return nil, ErrBinding
	}
	// ListenUnix's birth mode comes from the original root caller's umask.
	// Ownership, original inode continuity and the exact promoted mode are
	// independently checked; an assumed umask cannot replace those observations.
	var born InvocationRecord
	if records.position >= len(lines) || json.Unmarshal(lines[records.position], &born) != nil || born.Mode > 0777 {
		return nil, ErrBinding
	}
	if next("socket-born", socket, "", born.Mode, 0, true) != nil ||
		next("socket-ready", socket, "", 0660, gid, true) != nil ||
		file("start-socket.json", socketRecord) != nil ||
		next("start-attempt", directory, guard, 0, 0, false) != nil ||
		next("invocation-observed", directory, guard, 0, 0, false) != nil ||
		next("completion-received", directory, frame, 0, 0, false) != nil ||
		file("start-archived.json", archive) != nil ||
		next("archived", filepath.Join(directory, "start-archived.json"), archive, 0600, 0, true) != nil ||
		next("ack-attempt", directory, frame, 0, 0, false) != nil ||
		next("ack-written", directory, frame, 0, 0, false) != nil || records.position != len(lines) {
		return nil, ErrBinding
	}
	return records.owned, nil
}
