package forwarding

import (
	"bytes"
	"encoding/json"
	"path/filepath"
)

// A completed start is the entire original ordered transcript, not an archive
// or ACK searched out of a partial journal. The returned native identities are
// evidence for independent descriptor comparison; they confer no live rights.
func decodeStartRecords(raw []byte, number attemptNumber, journal, directory, binding, guard, socketRecord, archive, frame string, gid uint32) (map[string]startEntry, error) {
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' || gid < 100 || gid > 999 || number >= attemptLimit {
		return nil, ErrBinding
	}
	for _, hash := range []string{binding, guard, socketRecord, archive, frame} {
		if !validStartDigest(hash) {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 25 {
		return nil, ErrBinding
	}
	position := 0
	schema, firstPhase := "ardents-forwarding-start-journal-v1", "first-start"
	if number != 0 {
		firstPhase = "invocation"
	}
	owned := make(map[string]startEntry)
	next := func(phase, path, hash string, mode, group uint32, native bool) error {
		if position >= len(lines) || len(lines[position])+1 > 32<<10 {
			return ErrBinding
		}
		var actual startEntry
		if json.Unmarshal(lines[position], &actual) != nil {
			return ErrBinding
		}
		canonical, err := json.Marshal(actual)
		if err != nil || !bytes.Equal(canonical, lines[position]) {
			return ErrBinding
		}
		wanted := startEntry{Schema: schema, Sequence: uint32(position), Phase: phase, Path: path, SHA256: hash, Mode: mode, GID: group}
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return ErrBinding
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
			if old, exists := owned[path]; exists && (old.Device != actual.Device || old.Inode != actual.Inode) {
				return ErrBinding
			}
		}
		if actual != wanted {
			return ErrBinding
		}
		if native {
			owned[path] = actual
		}
		position++
		return nil
	}
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
	pending := digest([]byte("ardents-forwarding-start-pending-v1\n" + binding + "\n"))
	if file("forwarding-start-pending", pending) != nil || file("start-guard.json", guard) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
		return nil, ErrBinding
	}
	// ListenUnix's birth mode comes from the original root caller's umask.
	// Ownership, original inode continuity and the exact promoted mode are
	// independently checked; an assumed umask cannot replace those observations.
	var born startEntry
	if position >= len(lines) || json.Unmarshal(lines[position], &born) != nil || born.Mode > 0777 {
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
		next("ack-written", directory, frame, 0, 0, false) != nil || position != len(lines) {
		return nil, ErrBinding
	}
	return owned, nil
}
