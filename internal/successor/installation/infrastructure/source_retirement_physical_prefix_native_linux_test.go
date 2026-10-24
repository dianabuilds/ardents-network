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

func sourceRetirementPhysicalCount(window string) int {
	switch window {
	case "native-before-joined":
		return 6
	case "result-before-close", "pending-before-unlink":
		return 12
	default:
		return 0
	}
}

// Outside observation only. A six-record prefix describes a recorded domain
// frame, not native completion. Even all twelve records remain unusable while
// pending: this reader grants no committed retirement, Start or continuation.
func sourceRetirementPhysicalPrefix(raw []byte, number sourceAttemptNumber, root, start, challenge, completed, result string, gid uint32, window string) ([]sourceStartEntry, error) {
	count := sourceRetirementPhysicalCount(window)
	if count == 0 || number >= sourceAttemptLimit || !sourcePath(root) || gid < 100 || gid > 999 || len(raw) == 0 || len(raw) > 128<<10 || raw[len(raw)-1] != '\n' {
		return nil, errors.New("exact finite original retirement prefix required")
	}
	for _, hash := range []string{start, challenge, completed} {
		if !sourceCutObserverSHA(hash) {
			return nil, errors.New("original retirement frame commitments required")
		}
	}
	if count == 6 && result != "" || count == 12 && !sourceCutObserverSHA(result) {
		return nil, errors.New("distinct retirement result frontier required")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte{'\n'})
	if len(lines) != count {
		return nil, errors.New("retirement physical prefix count differs")
	}
	directory := number.directory(root)
	if count == 12 {
		// Reuse the unchanged complete grammar without bypassing its separate
		// pending/native/history/authority owners or synthesizing a suffix.
		if _, err := decodeSourceRetirementRecords(raw, directory, start, challenge, completed, result, gid); err != nil {
			return nil, err
		}
	}
	entries := make([]sourceStartEntry, count)
	for index, line := range lines {
		if len(line)+1 > 32<<10 || json.Unmarshal(line, &entries[index]) != nil {
			return nil, errors.New("bounded original retirement record required")
		}
		encoded, err := json.Marshal(entries[index])
		if err != nil || !bytes.Equal(line, encoded) {
			return nil, errors.Join(errors.New("canonical retirement record required"), err)
		}
	}
	if count == 12 {
		return entries, nil
	}
	socket := filepath.Join(directory, "stop-completion.socket")
	wanted := []sourceStartEntry{
		{Phase: "stop", Path: filepath.Join(directory, "source-retirement-journal"), SHA256: start, Mode: 0600},
		{Phase: "socket-intent", Path: socket},
		{Phase: "socket-born", Path: socket, Mode: entries[2].Mode},
		{Phase: "socket-ready", Path: socket, Mode: 0660, GID: gid},
		{Phase: "interrupt-attempt", Path: directory, SHA256: challenge},
		{Phase: "domain-joined", Path: directory, SHA256: completed},
	}
	for index := range wanted {
		wanted[index].Schema, wanted[index].Sequence = "ardents-source-retirement-record-v1", uint32(index)
		if index == 0 || index == 2 || index == 3 {
			if entries[index].Device == 0 || entries[index].Inode == 0 {
				return nil, errors.New("original retirement birth identity required")
			}
			wanted[index].Device, wanted[index].Inode = entries[index].Device, entries[index].Inode
		}
		if entries[index] != wanted[index] || index == 2 && entries[index].Mode > 0777 {
			return nil, errors.New("original retirement phase/native declaration differs")
		}
	}
	if entries[2].Device != entries[3].Device || entries[2].Inode != entries[3].Inode {
		return nil, errors.New("original retirement socket birth changed")
	}
	return entries, nil
}

// Native image of an independently checked prefix. Socket metadata is never
// read as file bytes. Pending and all original file identities remain required
// even at a complete-looking result or after the outside writer has closed.
func sourceRetirementPhysicalFiles(records []sourceStartEntry, gid uint32) ([]sourcePhysicalSealedFile, error) {
	if len(records) != 6 && len(records) != 12 || gid < 100 || gid > 999 {
		return nil, errors.New("checked finite retirement prefix required")
	}
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
			return errors.New("original retirement native resource identity/access differs")
		}
		result = append(result, sourcePhysicalSealedFile{Record: sourceRecord{Path: record.Path, Device: record.Device, Inode: record.Inode, SHA256: pin, Mode: record.Mode, UID: record.UID, GID: record.GID}, Size: info.Size(), Changed: native.Ctim, Modified: native.Mtim})
		return nil
	}
	for index, record := range records {
		if index == 0 || record.Phase == "file-ready" {
			raw, info, err := readSourceCutObserverFile(record.Path, 128<<10)
			if err != nil || index == 0 && !bytes.Equal(raw, canonical) || index != 0 && sourcePhysicalSHA(raw) != record.SHA256 {
				return nil, errors.Join(errors.New("original retirement resource bytes differ"), err)
			}
			if err := appendNative(record, info, sourcePhysicalSHA(raw)); err != nil {
				return nil, err
			}
		} else if record.Phase == "socket-ready" {
			info, err := os.Lstat(record.Path)
			if err != nil || info.Mode()&os.ModeSocket == 0 {
				return nil, errors.Join(errors.New("original retirement native socket required"), err)
			}
			if err := appendNative(record, info, ""); err != nil {
				return nil, err
			}
		}
	}
	directory := records[4].Path
	info, err := os.Lstat(directory)
	if err != nil || !sourceAccess(info, 0750, 0, gid, true) {
		return nil, errors.Join(errors.New("original retirement directory access differs"), err)
	}
	native := info.Sys().(*syscall.Stat_t)
	if err := appendNative(sourceStartEntry{Path: directory, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0750, GID: gid}, info, ""); err != nil {
		return nil, err
	}
	pending := filepath.Join(directory, "source-retirement-pending")
	raw, info, err := readSourceCutObserverFile(pending, 4096)
	if err != nil || !bytes.Equal(raw, []byte("ardents-source-retirement-pending-v1\n"+records[0].SHA256+"\n")) {
		return nil, errors.Join(errors.New("original retirement pending commitment differs"), err)
	}
	native = info.Sys().(*syscall.Stat_t)
	if err := appendNative(sourceStartEntry{Path: pending, Device: uint64(native.Dev), Inode: native.Ino, Mode: 0600}, info, sourcePhysicalSHA(raw)); err != nil {
		return nil, err
	}
	if len(records) == 6 {
		if _, err := os.Lstat(filepath.Join(directory, "source-retirement.json")); !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(errors.New("unrecorded retirement result must remain absent"), err)
		}
	}
	return result, nil
}
