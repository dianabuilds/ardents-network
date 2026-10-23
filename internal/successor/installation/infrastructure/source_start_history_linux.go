package infrastructure

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/completion"
)

type sourceStartHistory struct {
	number  sourceAttemptNumber
	archive sourceStartArchive
	id      [16]byte
	sha256  string
}

// This independent root reader consumes every ordered writer record and its
// original named resources. It grants neither fresh Release admission nor
// domain/native retirement. Its descriptors belong to this new root operation.
func readSourceStartHistory(selection *sourceSelection, number sourceAttemptNumber, initialHistory, predecessor string) (*sourceStartHistory, error) {
	if selection == nil || os.Geteuid() != 0 || number >= sourceAttemptLimit || !sourceInitialSHA(initialHistory) || (number == 0 && predecessor != "") || (number != 0 && !sourceInitialSHA(predecessor)) {
		return nil, ErrInput
	}
	b := selection.binding
	root := b.Request.InstallationRoot
	directory := number.directory(root)
	if _, err := os.Lstat(filepath.Join(directory, "source-start-pending")); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	if err := selection.pinDirectory(directory, 0750, 0, b.GID); err != nil {
		return nil, err
	}
	get := func(name string) ([]byte, error) {
		return selection.readPinned(filepath.Join(directory, name), 64<<10, 0600, 0, 0)
	}
	guard, err := get("start-guard.json")
	if err != nil {
		return nil, err
	}
	var intent sourceStartIntent
	if !sourceCanonicalRecord(guard, &intent) {
		return nil, ErrBinding
	}
	_, _, intentSchema, archiveSchema := number.schema()
	binding := digest(selection.bindingBytes())
	if intent.Schema != intentSchema || intent.BindingSHA256 != binding || intent.InitialHistorySHA256 != initialHistory || !sourceInitialSHA(intent.TargetsSHA256) || intent.UID != b.UID || intent.GID != b.GID || intent.Attempt != uint32(number) || intent.PredecessorSHA256 != predecessor {
		return nil, ErrBinding
	}
	socketRaw, err := get("start-socket.json")
	if err != nil {
		return nil, err
	}
	var socket sourceSocketRecord
	wantedSocketSchema := "ardents-source-first-start-socket-v1"
	if number != 0 {
		wantedSocketSchema = "ardents-source-invocation-socket-v1"
	}
	if !sourceCanonicalRecord(socketRaw, &socket) || socket.Schema != wantedSocketSchema || socket.GuardSHA256 != digest(guard) || socket.GID != b.GID || socket.Device == 0 || socket.Inode == 0 {
		return nil, ErrBinding
	}
	archiveRaw, err := get("start-archived.json")
	if err != nil {
		return nil, err
	}
	h := &sourceStartHistory{number: number, sha256: digest(archiveRaw)}
	if !sourceCanonicalRecord(archiveRaw, &h.archive) {
		return nil, ErrBinding
	}
	a := h.archive
	id, err := hex.DecodeString(a.InvocationID)
	if err != nil || len(id) != 16 || hex.EncodeToString(id) != a.InvocationID {
		return nil, ErrBinding
	}
	copy(h.id[:], id)
	if a.Schema != archiveSchema || a.BindingSHA256 != binding || a.GuardSHA256 != digest(guard) || a.Generation != b.Generation || a.PID == 0 || h.id == [16]byte{} || a.ProcessDevice == 0 || a.ProcessInode == 0 || a.CgroupDevice == 0 || a.CgroupInode == 0 || a.Manager == "" || a.StartedRealtime == 0 || a.StartedMonotonic == 0 {
		return nil, ErrBinding
	}
	frame, err := completion.Encode(h.id, b.Generation, binding)
	if err != nil {
		return nil, err
	}
	journal := number.journal(root)
	raw, err := selection.read(journal, 128<<10, 0600, 0, 0)
	if err != nil {
		return nil, err
	}
	recorded, err := decodeSourceStartRecords(raw, number, journal, directory, binding, digest(guard), digest(socketRaw), digest(archiveRaw), digest(frame[:]), b.GID)
	if err != nil {
		return nil, err
	}
	for path, record := range recorded {
		var info os.FileInfo
		switch path {
		case directory:
			info = selection.identities[directory]
		case filepath.Join(directory, "start-completion.socket"):
			info, err = os.Lstat(path)
			if err != nil || !sameSourceSocket(info, info, 0660, b.GID) {
				return nil, errors.Join(ErrBinding, err)
			}
		default:
			file := selection.files[path]
			if file == nil {
				return nil, ErrBinding
			}
			info = file.info
		}
		if info == nil {
			return nil, ErrBinding
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || uint64(native.Dev) != record.Device || native.Ino != record.Inode || uint32(info.Mode().Perm()) != record.Mode || native.Uid != record.UID || native.Gid != record.GID {
			return nil, ErrBinding
		}
	}
	if socket.Device != recorded[filepath.Join(directory, "start-completion.socket")].Device || socket.Inode != recorded[filepath.Join(directory, "start-completion.socket")].Inode {
		return nil, ErrBinding
	}
	if err := selection.observe(); err != nil {
		return nil, err
	}
	if _, err := os.Lstat(filepath.Join(directory, "source-start-pending")); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	return h, nil
}

func sourceCanonicalRecord(raw []byte, target any) bool {
	if len(raw) == 0 || len(raw) > 64<<10 || raw[len(raw)-1] != '\n' || json.Unmarshal(raw, target) != nil {
		return false
	}
	canonical, err := json.Marshal(target)
	return err == nil && bytes.Equal(raw, append(canonical, '\n'))
}
