package journal

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"strings"
)

// A completed start is the entire original ordered transcript, not an archive
// or ACK searched out of a partial journal. The returned native identities are
// evidence for independent descriptor comparison; they confer no live rights.
func DecodeStartRecords(raw []byte, purpose string, number uint32, journal, directory, binding, guard, socketRecord, archive, frame, acknowledgement string, gid uint32) (map[string]InvocationRecord, error) {
	if purpose != "join" && purpose != "descriptor" {
		return nil, ErrBinding
	}
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' || gid < 100 || gid > 999 || number >= 64 {
		return nil, ErrBinding
	}
	for _, hash := range []string{binding, guard, socketRecord, archive, frame, acknowledgement} {
		if !validInvocationDigest(hash) {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 25 && len(lines) != 26 {
		return nil, ErrBinding
	}
	schema, firstPhase := "ardents-"+purpose+"-start-journal-v1", "first-start"
	if number != 0 {
		firstPhase = "invocation"
	}
	records := invocationRecords{lines: lines, schema: schema, owned: make(map[string]InvocationRecord)}
	next := records.next
	if next(firstPhase, journal, binding, 0600, 0, true) != nil {
		return nil, ErrBinding
	}
	if len(lines) == 26 {
		var context InvocationRecord
		if json.Unmarshal(lines[records.position], &context) != nil || !validInvocationDigest(context.SHA256) {
			return nil, ErrBinding
		}
		if next("native-context", filepath.Dir(directory), context.SHA256, 0, 0, false) != nil {
			return nil, ErrBinding
		}
	}
	if next("directory-intent", directory, "", 0, 0, false) != nil ||
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
	pending := invocationDigest([]byte("ardents-" + purpose + "-start-pending-v1\n" + binding + "\n"))
	if file(purpose+"-start-pending", pending) != nil || file("start-guard.json", guard) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
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
		next("ack-attempt", directory, acknowledgement, 0, 0, false) != nil ||
		next("ack-written", directory, acknowledgement, 0, 0, false) != nil || records.position != len(lines) {
		return nil, ErrBinding
	}
	return records.owned, nil
}

func validInvocationDigest(value string) bool {
	return len(value) == 64 && strings.Trim(value, "0123456789abcdef") == ""
}
func invocationDigest(raw []byte) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
