package infrastructure

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
)

// Only root's explicit-stop producer writes this original completion. The
// domain frame, exact archive, native exit and ordered physical-close journal
// are all required; none is independently a restart token.
type sourceRetirementResult struct {
	Schema           string `json:"schema"`
	Attempt          uint32 `json:"attempt"`
	Generation       string `json:"generation"`
	BindingSHA256    string `json:"binding_sha256"`
	StartSHA256      string `json:"start_sha256"`
	InvocationID     string `json:"invocation_id"`
	PID              uint32 `json:"pid"`
	Manager          string `json:"manager"`
	Challenge        string `json:"challenge"`
	CompletionSHA256 string `json:"completion_sha256"`
	StartedRealtime  uint64 `json:"started_realtime"`
	StartedMonotonic uint64 `json:"started_monotonic"`
	ExitedRealtime   uint64 `json:"exited_realtime"`
	ExitedMonotonic  uint64 `json:"exited_monotonic"`
	Code             int32  `json:"code"`
	Status           int32  `json:"status"`
}

func (r sourceRetirementResult) execution() sourceExecution {
	return sourceExecution{r.StartedRealtime, r.StartedMonotonic, r.ExitedRealtime, r.ExitedMonotonic, r.PID, r.Code, r.Status}
}

func decodeSourceRetirementRecords(raw []byte, directory, start, challenge, completed, result string, gid uint32) (map[string]sourceStartEntry, error) {
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' || gid < 100 || gid > 999 {
		return nil, ErrBinding
	}
	for _, hash := range []string{start, challenge, completed, result} {
		if !sourceInitialSHA(hash) {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 12 {
		return nil, ErrBinding
	}
	position := 0
	owned := map[string]sourceStartEntry{}
	next := func(phase, path, sha string, mode, group uint32, native bool) error {
		if position >= len(lines) || len(lines[position])+1 > 32<<10 {
			return ErrBinding
		}
		var actual sourceStartEntry
		if json.Unmarshal(lines[position], &actual) != nil {
			return ErrBinding
		}
		canonical, err := json.Marshal(actual)
		if err != nil || !bytes.Equal(canonical, lines[position]) {
			return ErrBinding
		}
		wanted := sourceStartEntry{Schema: "ardents-source-retirement-record-v1", Sequence: uint32(position), Phase: phase, Path: path, SHA256: sha, Mode: mode, GID: group}
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return ErrBinding
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
			if previous, found := owned[path]; found && (previous.Device != actual.Device || previous.Inode != actual.Inode) {
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
	journal := filepath.Join(directory, "source-retirement-journal")
	socket := filepath.Join(directory, "stop-completion.socket")
	file := filepath.Join(directory, "source-retirement.json")
	if next("stop", journal, start, 0600, 0, true) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
		return nil, ErrBinding
	}
	var birth sourceStartEntry
	if json.Unmarshal(lines[position], &birth) != nil || birth.Mode > 0777 {
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
		next("retired", file, result, 0600, 0, true) != nil || position != len(lines) {
		return nil, ErrBinding
	}
	return owned, nil
}

func sourceRetirementChallenge(text string) ([32]byte, error) {
	var result [32]byte
	raw, err := hex.DecodeString(text)
	if err != nil || len(raw) != len(result) || hex.EncodeToString(raw) != text {
		return result, ErrBinding
	}
	copy(result[:], raw)
	if result == [32]byte{} {
		return result, ErrBinding
	}
	return result, nil
}
