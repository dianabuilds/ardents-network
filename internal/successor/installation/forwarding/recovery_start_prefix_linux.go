package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Any sidecar requires the complete independent recovery reader. Incomplete
// publication must never fall back to the ordinary completed-start reader.
func startRecoveryPresent(journal string) (bool, error) {
	control, result, pending := transitionCutPaths(journal)
	recovery, completion, guard := recoveryPaths(journal)
	before, after := recoveryReloadPaths(journal)
	present := false
	for _, path := range []string{control, result, pending, recovery, completion, guard, journal + ".recovery-native-context", before, after} {
		if _, err := os.Lstat(path); err == nil {
			present = true
		} else if !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return present, nil
}

type neverStartedPrefix struct {
	number  attemptNumber
	intent  []byte
	history transitionStartIntent
	rows    []initialRecord
	owned   map[string]initialRecord
	bodies  map[string][]byte
}

func startJournalNumber(root, journal string) (attemptNumber, error) {
	for n := attemptNumber(0); n < attemptLimit; n++ {
		if n.journal(root) == journal {
			return n, nil
		}
	}
	return 0, ErrBinding
}

// No effect or chain acceptance follows from these detached declarations.
func originalStartIntent(raw []byte, selected generationSelection, number attemptNumber) (transitionStartIntent, error) {
	var header struct {
		Initial     string `json:"initial_history_sha256"`
		Predecessor string `json:"predecessor_sha256"`
	}
	if len(raw) == 0 || len(raw) > 8<<10 || json.Unmarshal(raw, &header) != nil || !validStartDigest(header.Initial) {
		return transitionStartIntent{}, ErrBinding
	}
	history, _, err := readStartHistoryIntent(raw, selected.binding, digest(selected.raw), header.Initial, number, header.Predecessor)
	if err != nil {
		return transitionStartIntent{}, err
	}
	return transitionStartIntent{initial: header.Initial, predecessor: header.Predecessor, number: number, history: history}, nil
}

// Every admitted original boundary is before manager.Start. Only an actual
// from-birth denied/joined cut makes that source ordering a usable fact.
func classifyNeverStarted(prefix []byte, cut transitionCutResult, selected generationSelection, legacyGuard []byte) (*neverStartedPrefix, error) {
	rows, err := decodeTransitionPrefix(prefix, "start")
	if err != nil || cut.Boundary.Operation != "start" || !bytes.Equal(cut.Boundary.Binding, selected.raw) || cut.Origin.ProgramSHA256 != selected.binding.ProgramSHA256 || !bytes.Equal(cut.Boundary.OriginalContext, cut.Boundary.NativeContext) {
		return nil, errors.Join(ErrRepairRequired, err)
	}
	native, err := decodeNativeContext(cut.Boundary.NativeContext)
	if err != nil || native.Manager != cut.Boundary.Manager {
		return nil, errors.Join(ErrBinding, err)
	}
	number, err := startJournalNumber(selected.binding.Request.InstallationRoot, cut.Boundary.Journal)
	if err != nil {
		return nil, err
	}
	intent := cut.Boundary.StartIntent
	contextRow := cut.Boundary.Schema == "ardents-forwarding-transition-boundary-v2" || len(rows) > 1 && rows[1].Phase == "native-context"
	if cut.Boundary.Schema == "ardents-forwarding-transition-boundary-v1" {
		ready := 9
		if contextRow {
			ready++
		}
		if len(rows) <= ready || rows[ready].Phase != "file-ready" || rows[ready].Path != filepath.Join(number.directory(selected.binding.Request.InstallationRoot), "start-guard.json") || rows[ready].SHA256 != digest(legacyGuard) {
			return nil, ErrRepairRequired
		}
		intent = legacyGuard
	} else if _, err := readTransitionStartIntent(cut.Boundary); err != nil {
		return nil, err
	}
	history, err := originalStartIntent(intent, selected, number)
	if err != nil {
		return nil, err
	}
	s := &neverStartedPrefix{number: number, intent: bytes.Clone(intent), history: history, rows: rows, owned: map[string]initialRecord{}, bodies: map[string][]byte{}}
	b := selected.binding
	root, dir := b.Request.InstallationRoot, number.directory(b.Request.InstallationRoot)
	position := 0
	identities := map[[2]uint64]string{}
	next := func(phase, path, sha string, mode, gid uint32, native bool) error {
		if position >= len(rows) {
			return nil
		}
		actual := rows[position]
		wanted := initialRecord{Sequence: position, Phase: phase, Path: path, SHA256: sha, Mode: mode, GID: gid}
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return ErrBinding
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
			if old, ok := s.owned[path]; ok && (old.Device != actual.Device || old.Inode != actual.Inode) {
				return ErrBinding
			}
			key := [2]uint64{actual.Device, actual.Inode}
			if old, ok := identities[key]; ok && old != path {
				return ErrBinding
			}
			identities[key] = path
		}
		if actual != wanted {
			return ErrBinding
		}
		if native {
			s.owned[path] = actual
		}
		position++
		return nil
	}
	first := "first-start"
	if number != 0 {
		first = "invocation"
	}
	if err := next(first, number.journal(root), digest(selected.raw), 0600, 0, true); err != nil {
		return nil, err
	}
	if contextRow {
		if err := next("native-context", root, digest(cut.Boundary.OriginalContext), 0, 0, false); err != nil {
			return nil, err
		}
	}
	if err := errors.Join(next("directory-intent", dir, "", 0, 0, false), next("directory-born", dir, "", 0700, 0, true), next("directory-ready", dir, "", 0750, b.GID, true)); err != nil {
		return nil, err
	}
	file := func(name string, body []byte) error {
		path := filepath.Join(dir, name)
		s.bodies[path] = bytes.Clone(body)
		sha := digest(body)
		return errors.Join(next("file-intent", path, sha, 0, 0, false), next("file-born", path, sha, 0600, 0, true), next("file-ready", path, sha, 0600, 0, true))
	}
	if err := file("forwarding-start-pending", []byte("ardents-forwarding-start-pending-v1\n"+digest(selected.raw)+"\n")); err != nil {
		return nil, err
	}
	if err := file("start-guard.json", intent); err != nil {
		return nil, err
	}
	socket := filepath.Join(dir, "start-completion.socket")
	if err := next("socket-intent", socket, "", 0, 0, false); err != nil {
		return nil, err
	}
	mode := uint32(0)
	if position < len(rows) {
		mode = rows[position].Mode
		if mode > 0777 {
			return nil, ErrBinding
		}
	}
	if err := errors.Join(next("socket-born", socket, "", mode, 0, true), next("socket-ready", socket, "", 0660, b.GID, true)); err != nil {
		return nil, err
	}
	object := s.owned[socket]
	data, err := json.Marshal(startSocketRecord{"ardents-forwarding-start-socket-v1", digest(intent), object.Device, object.Inode, b.GID})
	if err != nil {
		return nil, err
	}
	if err := file("start-socket.json", append(data, '\n')); err != nil {
		return nil, err
	}
	if err := next("start-attempt", dir, digest(intent), 0, 0, false); err != nil {
		return nil, err
	}
	if position != len(rows) {
		return nil, ErrRepairRequired
	}
	return s, nil
}

func (s *neverStartedPrefix) clear() {
	if s != nil {
		clear(s.intent)
		for _, raw := range s.bodies {
			clear(raw)
		}
	}
}
