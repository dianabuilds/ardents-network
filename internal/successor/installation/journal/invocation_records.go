package journal

import (
	"bytes"
	"encoding/json"
)

// InvocationRecord describes one detached canonical journal record; it carries no
// native custody, Release permission or evidence of a live invocation.
type InvocationRecord struct {
	Schema   string `json:"schema"`
	Sequence uint32 `json:"sequence"`
	Phase    string `json:"phase"`
	Path     string `json:"path"`
	Device   uint64 `json:"device"`
	Inode    uint64 `json:"inode"`
	SHA256   string `json:"sha256"`
	Mode     uint32 `json:"mode"`
	UID      uint32 `json:"uid"`
	GID      uint32 `json:"gid"`
}

// invocationRecords verifies ordered canonical records and retains only their
// detached inode facts. The caller owns phase grammar and native custody.
type invocationRecords struct {
	lines    [][]byte
	schema   string
	position int
	owned    map[string]InvocationRecord
}

func (r *invocationRecords) next(phase, path, hash string, mode, group uint32, native bool) error {
	if r.position >= len(r.lines) || len(r.lines[r.position])+1 > 32<<10 {
		return ErrBinding
	}
	var actual InvocationRecord
	if json.Unmarshal(r.lines[r.position], &actual) != nil {
		return ErrBinding
	}
	canonical, err := json.Marshal(actual)
	if err != nil || !bytes.Equal(canonical, r.lines[r.position]) {
		return ErrBinding
	}
	wanted := InvocationRecord{Schema: r.schema, Sequence: uint32(r.position), Phase: phase, Path: path, SHA256: hash, Mode: mode, GID: group}
	if native {
		if actual.Device == 0 || actual.Inode == 0 {
			return ErrBinding
		}
		wanted.Device, wanted.Inode = actual.Device, actual.Inode
		if previous, exists := r.owned[path]; exists && (previous.Device != actual.Device || previous.Inode != actual.Inode) {
			return ErrBinding
		}
	}
	if actual != wanted {
		return ErrBinding
	}
	if native {
		r.owned[path] = actual
	}
	r.position++
	return nil
}
