//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Outside process-cut observation only. This exact partial grammar does not
// enter the product's complete-only reader and grants no Start/ACK, original
// process join, Source authority or recovery continuation.
func sourceStartupPhysicalPrefix(raw []byte, number sourceAttemptNumber, root, binding, guard, socketRecord, archive, frame string, gid uint32, window string) ([]sourceStartEntry, error) {
	count := 19
	if window == "ack-before-written" {
		count = 21
	} else if window != "archive-before-archived" {
		return nil, errors.New("finite startup physical window required")
	}
	if number >= sourceAttemptLimit || !sourcePath(root) || gid < 100 || gid > 999 || len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' {
		return nil, errors.New("exact original startup prefix required")
	}
	for _, pin := range []string{binding, guard, socketRecord, archive, frame} {
		if !sourceCutObserverSHA(pin) {
			return nil, errors.New("original startup public commitments required")
		}
	}
	schema, first, _, _ := number.schema()
	directory, journal := number.directory(root), number.journal(root)
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != count {
		return nil, errors.New("startup physical prefix count differs")
	}
	position := 0
	var result []sourceStartEntry
	owned := map[string]sourceStartEntry{}
	next := func(phase, path, pin string, mode, group uint32, native bool) error {
		if position >= len(lines) || len(lines[position])+1 > 32<<10 {
			return errors.New("bounded startup physical record required")
		}
		var actual sourceStartEntry
		if err := json.Unmarshal(lines[position], &actual); err != nil {
			return err
		}
		canonical, err := json.Marshal(actual)
		if err != nil || !bytes.Equal(canonical, lines[position]) {
			return errors.Join(errors.New("canonical startup physical record required"), err)
		}
		wanted := sourceStartEntry{Schema: schema, Sequence: uint32(position), Phase: phase, Path: path, SHA256: pin, Mode: mode, GID: group}
		if native {
			if actual.Device == 0 || actual.Inode == 0 {
				return errors.New("original startup native identity required")
			}
			wanted.Device, wanted.Inode = actual.Device, actual.Inode
			if previous, exists := owned[path]; exists && (previous.Device != actual.Device || previous.Inode != actual.Inode) {
				return errors.New("startup original birth identity changed")
			}
		}
		if actual != wanted {
			return errors.New("startup physical phase or native declaration differs")
		}
		if native {
			owned[path] = actual
		}
		result, position = append(result, actual), position+1
		return nil
	}
	if next(first, journal, binding, 0600, 0, true) != nil ||
		next("directory-intent", directory, "", 0, 0, false) != nil ||
		next("directory-born", directory, "", 0700, 0, true) != nil ||
		next("directory-ready", directory, "", 0750, gid, true) != nil {
		return nil, errors.New("original startup journal/directory prefix differs")
	}
	file := func(name, pin string) error {
		path := filepath.Join(directory, name)
		if next("file-intent", path, pin, 0, 0, false) != nil ||
			next("file-born", path, pin, 0600, 0, true) != nil ||
			next("file-ready", path, pin, 0600, 0, true) != nil {
			return errors.New("original startup file birth triple differs")
		}
		return nil
	}
	socket := filepath.Join(directory, "start-completion.socket")
	if file("start-guard.json", guard) != nil || next("socket-intent", socket, "", 0, 0, false) != nil {
		return nil, errors.New("startup guard/socket intent differs")
	}
	var born sourceStartEntry
	if json.Unmarshal(lines[position], &born) != nil || born.Mode > 0777 {
		return nil, errors.New("exact native startup socket birth mode required")
	}
	if next("socket-born", socket, "", born.Mode, 0, true) != nil ||
		next("socket-ready", socket, "", 0660, gid, true) != nil ||
		file("start-socket.json", socketRecord) != nil ||
		next("start-attempt", directory, guard, 0, 0, false) != nil ||
		next("invocation-observed", directory, guard, 0, 0, false) != nil ||
		next("completion-received", directory, frame, 0, 0, false) != nil ||
		file("start-archived.json", archive) != nil {
		return nil, errors.New("actual original startup archive prefix differs")
	}
	if window == "ack-before-written" && (next("archived", filepath.Join(directory, "start-archived.json"), archive, 0600, 0, true) != nil ||
		next("ack-attempt", directory, frame, 0, 0, false) != nil) {
		return nil, errors.New("original archived/ACK intent prefix differs")
	}
	if position != count {
		return nil, errors.New("startup physical prefix has unobserved records")
	}
	return result, nil
}

// Read actual original native resources named by an independently checked
// prefix. A socket is metadata, not readable file data; all byte-bearing
// resources are bounded root0600 files. This image proves neither archive
// semantics nor that the separately owned PID1 Source process has joined.
func sourceStartupPhysicalFiles(records []sourceStartEntry) ([]sourcePhysicalSealedFile, error) {
	if len(records) != 19 && len(records) != 21 {
		return nil, errors.New("checked startup physical prefix required")
	}
	return sourceStartupNativeResources(records, true)
}

// A distinct complete image for retirement observation, never permission to
// accept partial startup. The caller first checks the unchanged full grammar.
func sourceStartupCompletedPhysicalFiles(records []sourceStartEntry) ([]sourcePhysicalSealedFile, error) {
	if len(records) != 22 {
		return nil, errors.New("checked complete startup transcript required")
	}
	return sourceStartupNativeResources(records, false)
}

func sourceStartupNativeResources(records []sourceStartEntry, pendingRequired bool) ([]sourcePhysicalSealedFile, error) {
	var canonical []byte
	for _, record := range records {
		raw, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		canonical = append(append(canonical, raw...), '\n')
	}
	var result []sourcePhysicalSealedFile
	appendNative := func(record sourceStartEntry, info os.FileInfo, pin string) error {
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode || uint32(info.Mode().Perm()) != record.Mode || native.Uid != record.UID || native.Gid != record.GID || !info.IsDir() && native.Nlink != 1 {
			return errors.New("original startup resource native identity/access differs")
		}
		result = append(result, sourcePhysicalSealedFile{Record: sourceRecord{Path: record.Path, Device: record.Device, Inode: record.Inode, SHA256: pin, Mode: record.Mode, UID: record.UID, GID: record.GID}, Size: info.Size(), Changed: native.Ctim, Modified: native.Mtim})
		return nil
	}
	for index, record := range records {
		switch {
		case index == 0 || record.Phase == "file-ready":
			raw, info, err := readSourceCutObserverFile(record.Path, 128<<10)
			if err != nil || index == 0 && !bytes.Equal(raw, canonical) || index != 0 && sourcePhysicalSHA(raw) != record.SHA256 {
				return nil, errors.Join(errors.New("original startup resource bytes differ"), err)
			}
			if err := appendNative(record, info, sourcePhysicalSHA(raw)); err != nil {
				return nil, err
			}
		case record.Phase == "directory-ready" || record.Phase == "socket-ready":
			info, err := os.Lstat(record.Path)
			if err != nil || record.Phase == "directory-ready" && !info.IsDir() || record.Phase == "socket-ready" && info.Mode()&os.ModeSocket == 0 {
				return nil, errors.Join(errors.New("original startup directory/socket kind differs"), err)
			}
			if err := appendNative(record, info, ""); err != nil {
				return nil, err
			}
		}
	}
	directory := records[3].Path
	pendingPath := filepath.Join(directory, "source-start-pending")
	if !pendingRequired {
		if _, err := os.Lstat(pendingPath); !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(errors.New("completed startup must remain unguarded"), err)
		}
		return result, nil
	}
	pending, info, err := readSourceCutObserverFile(pendingPath, 4096)
	if err != nil || !bytes.Equal(pending, []byte("ardents-source-start-pending-v1\n"+records[0].SHA256+"\n")) {
		return nil, errors.Join(errors.New("original startup pending marker differs"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	if err := appendNative(sourceStartEntry{Path: pendingPath, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}, info, sourcePhysicalSHA(pending)); err != nil {
		return nil, err
	}
	return result, nil
}
