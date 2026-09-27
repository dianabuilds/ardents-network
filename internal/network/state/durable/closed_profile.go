package durable

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
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
		if _, statErr := os.Lstat(filepath.Join(root.path, profileName)); statErr == nil {
			return ClosedProfileState{}, nil, errors.New("closed profile bytes lack durable state")
		} else if !os.IsNotExist(statErr) {
			return ClosedProfileState{}, nil, statErr
		}
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
	} else if os.IsNotExist(err) {
		if err := writeSynced(profilePath, profile); err != nil {
			return err
		}
		if err := syncDirectory(root.path); err != nil {
			return err
		}
	} else {
		return err
	}
	return replaceClosedProfileState(root.path, closedProfileStateName(state.Generation), encodeClosedProfileState(state))
}

func replaceClosedProfileState(root, name string, raw []byte) error {
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
	return syncDirectory(root)
}
