package durable

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const (
	closedProfileStateSize = 8 + 32 + 8 + 32 + 32
)

type ClosedProfileState struct {
	Generation, Accepted, Conflict [32]byte
	Epoch                          uint64
}

func (state ClosedProfileState) valid() bool {
	return state.Generation != [32]byte{} && state.Epoch != 0 && state.Accepted != [32]byte{} &&
		(state.Conflict == [32]byte{} || state.Conflict != state.Accepted)
}

func encodeClosedProfileState(state ClosedProfileState) []byte {
	raw := make([]byte, 0, closedProfileStateSize)
	raw = append(raw, "ARDCPST1"...)
	raw = append(raw, state.Generation[:]...)
	raw = binary.BigEndian.AppendUint64(raw, state.Epoch)
	raw = append(raw, state.Accepted[:]...)
	return append(raw, state.Conflict[:]...)
}

func decodeClosedProfileState(raw []byte) (ClosedProfileState, error) {
	if len(raw) != closedProfileStateSize || string(raw[:8]) != "ARDCPST1" {
		return ClosedProfileState{}, errors.New("closed profile state framing is invalid")
	}
	state := ClosedProfileState{}
	copy(state.Generation[:], raw[8:40])
	state.Epoch = binary.BigEndian.Uint64(raw[40:48])
	copy(state.Accepted[:], raw[48:80])
	copy(state.Conflict[:], raw[80:112])
	if !state.valid() {
		return ClosedProfileState{}, errors.New("closed profile state is invalid")
	}
	return state, nil
}

func closedProfileStateName(generation [32]byte) string {
	return fmt.Sprintf("closed-profile-state-%x", generation)
}

func closedProfileBytesName(generation [32]byte) string {
	return fmt.Sprintf("closed-profile-%x.bin", generation)
}

func (root *Root) LoadClosedProfile(generation [32]byte) (ClosedProfileState, []byte, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return ClosedProfileState{}, nil, err
	}
	stateName, profileName := closedProfileStateName(generation), closedProfileBytesName(generation)
	stateRaw, err := readBoundedFile(filepath.Join(root.path, stateName), closedProfileStateSize)
	if os.IsNotExist(err) {
		// A crash can leave the immutable bytes before the state record. They
		// are not accepted until State verifies an exact retry and CommitClosedProfile
		// confirms the on-disk bytes match before publishing the state.
		return ClosedProfileState{}, nil, nil
	}
	if err != nil {
		return ClosedProfileState{}, nil, fmt.Errorf("read closed profile state: %w", err)
	}
	state, err := decodeClosedProfileState(stateRaw)
	if err != nil {
		return ClosedProfileState{}, nil, err
	}
	if state.Generation != generation {
		return ClosedProfileState{}, nil, errors.New("closed profile state generation is wrong")
	}
	profile, err := readBoundedFile(filepath.Join(root.path, profileName), root.limits.ClosedProfileBytes)
	if err != nil || sha256.Sum256(profile) != state.Accepted {
		return ClosedProfileState{}, nil, errors.New("closed profile bytes disagree with durable state")
	}
	return state, profile, nil
}

func (root *Root) CommitClosedProfile(state ClosedProfileState, profile []byte) error {
	return root.commitClosedProfileWithOps(state, profile, standardClosedProfileByteOps())
}

func standardClosedProfileByteOps() closedProfileByteOps {
	return closedProfileByteOps{
		write: (*os.File).Write, syncFile: (*os.File).Sync, closeFile: (*os.File).Close,
		syncExisting: syncClosedProfileFile, syncDirectory: syncDirectory,
	}
}

// The byte-phase operations are supplied together so each failure boundary can
// be exercised without modifying the leased root or global filesystem state.
type closedProfileByteOps struct {
	write         func(*os.File, []byte) (int, error)
	syncFile      func(*os.File) error
	closeFile     func(*os.File) error
	syncExisting  func(string) error
	syncDirectory func(string) error
}

func (root *Root) commitClosedProfileWithOps(state ClosedProfileState, profile []byte, ops closedProfileByteOps) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil || !state.valid() || len(profile) == 0 || int64(len(profile)) > root.limits.ClosedProfileBytes || sha256.Sum256(profile) != state.Accepted {
		return errors.New("closed profile durable input is invalid")
	}
	profilePath := filepath.Join(root.path, closedProfileBytesName(state.Generation))
	if existing, err := readBoundedFile(profilePath, root.limits.ClosedProfileBytes); err == nil {
		if !bytes.Equal(existing, profile) {
			return errors.New("closed profile bytes are immutable")
		}
		if err := ops.syncExisting(profilePath); err != nil {
			return fmt.Errorf("sync existing closed profile bytes: %w", err)
		}
		if err := ops.syncDirectory(root.path); err != nil {
			return fmt.Errorf("sync existing closed profile directory: %w", err)
		}
	} else if os.IsNotExist(err) {
		if err := publishClosedProfileBytes(root.path, profilePath, profile, ops); err != nil {
			return err
		}
	} else {
		return err
	}
	return replaceClosedProfileState(root.path, closedProfileStateName(state.Generation), encodeClosedProfileState(state))
}

func publishClosedProfileBytes(rootPath, profilePath string, profile []byte, ops closedProfileByteOps) error {
	temporary, err := os.CreateTemp(rootPath, ".closed-profile-")
	if err != nil {
		return fmt.Errorf("create closed profile staging: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err = temporary.Chmod(0o600); err == nil {
		var written int
		written, err = ops.write(temporary, profile)
		if err == nil && written != len(profile) {
			err = io.ErrShortWrite
		}
	}
	if err == nil {
		err = ops.syncFile(temporary)
	}
	closeErr := ops.closeFile(temporary)
	if err != nil {
		return fmt.Errorf("write closed profile staging: %w", errors.Join(err, closeErr))
	}
	if closeErr != nil {
		return fmt.Errorf("close closed profile staging: %w", closeErr)
	}
	// A hard link publishes the synced inode without replacing a final path
	// that appeared after the earlier absence check.
	if err := os.Link(temporaryPath, profilePath); err != nil {
		return fmt.Errorf("publish closed profile bytes: %w", err)
	}
	if err := os.Remove(temporaryPath); err != nil {
		// The final bytes may be visible, but no state record has been written.
		// An exact retry must re-sync them before acceptance.
		return fmt.Errorf("remove closed profile staging: %w", err)
	}
	if err := ops.syncDirectory(rootPath); err != nil {
		return fmt.Errorf("sync closed profile bytes directory: %w", err)
	}
	return nil
}

func syncClosedProfileFile(path string) error {
	file, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	err = file.Sync()
	return errors.Join(err, file.Close())
}

// ErrClosedProfileStateSyncUncertain means the state record rename succeeded
// but its directory sync failed. The accepted or conflicting record may already
// be visible, so the live State owner must stop serving it until reopen verifies it.
var ErrClosedProfileStateSyncUncertain = errors.New("closed profile state durability is uncertain after rename")

func replaceClosedProfileState(root, name string, raw []byte) error {
	return replaceClosedProfileStateWithSync(root, name, raw, syncDirectory)
}

// The injected sync isolates the post-rename boundary in deterministic tests.
func replaceClosedProfileStateWithSync(root, name string, raw []byte, sync func(string) error) error {
	temporary, err := os.CreateTemp(root, ".closed-profile-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() { _ = os.Remove(temporaryPath) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(raw)
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(temporaryPath, filepath.Join(root, name)); err != nil {
		return err
	}
	if err := sync(root); err != nil {
		return fmt.Errorf("sync closed profile state: %w: %w", ErrClosedProfileStateSyncUncertain, err)
	}
	return nil
}
