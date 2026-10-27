package forwarding

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
)

type retirementResult struct {
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

func (r retirementResult) execution() forwardingExecution {
	return forwardingExecution{r.StartedRealtime, r.StartedMonotonic, r.ExitedRealtime, r.ExitedMonotonic, r.PID, r.Code, r.Status}
}

func retirementChallenge(text string) (result [32]byte, err error) {
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

// The entire native transcript is required. A domain frame, result file or
// native-closed record found in a partial prefix is not committed retirement.
func decodeRetirementRecords(raw []byte, journal, directory, start, challenge, completed, result string, gid uint32) (map[string]retirementEntry, error) {
	if len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' || gid < 100 || gid > 999 {
		return nil, ErrBinding
	}
	for _, hash := range []string{start, challenge, completed, result} {
		if !validStartDigest(hash) {
			return nil, ErrBinding
		}
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != 18 {
		return nil, ErrBinding
	}
	position := 0
	owned := map[string]retirementEntry{}
	next := func(phase, path, hash string, mode, group uint32, native bool) error {
		if position >= len(lines) || len(lines[position])+1 > 32<<10 {
			return ErrBinding
		}
		var actual retirementEntry
		if json.Unmarshal(lines[position], &actual) != nil {
			return ErrBinding
		}
		canonical, err := json.Marshal(actual)
		if err != nil || !bytes.Equal(canonical, lines[position]) {
			return ErrBinding
		}
		wanted := retirementEntry{Schema: "ardents-forwarding-retirement-journal-v1", Sequence: uint32(position), Phase: phase, Path: path, SHA256: hash, Mode: mode, GID: group}
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return ErrBinding
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
			if previous, exists := owned[path]; exists && (previous.Device != actual.Device || previous.Inode != actual.Inode) {
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
	pending := digest([]byte("ardents-forwarding-retirement-pending-v1\n" + start + "\n"))
	socket := filepath.Join(directory, "stop-completion.socket")
	if file("retirement-pending", pending) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
		return nil, ErrBinding
	}
	var born retirementEntry
	if position >= len(lines) || json.Unmarshal(lines[position], &born) != nil || born.Mode > 0777 {
		return nil, ErrBinding
	}
	if next("socket-born", socket, "", born.Mode, 0, true) != nil || next("socket-ready", socket, "", 0660, gid, true) != nil || next("interrupt-attempt", directory, challenge, 0, 0, false) != nil || next("domain-joined", directory, completed, 0, 0, false) != nil || next("native-joined", directory, result, 0, 0, false) != nil || next("native-closed", directory, result, 0, 0, false) != nil || file("retirement-result.json", result) != nil || next("retired", filepath.Join(directory, "retirement-result.json"), result, 0600, 0, true) != nil || position != len(lines) {
		return nil, ErrBinding
	}
	return owned, nil
}
