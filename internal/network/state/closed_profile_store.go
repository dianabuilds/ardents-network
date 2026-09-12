package state

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

type closedProfileState struct {
	generation, accepted, conflict [32]byte
	epoch                          uint64
}

func (state closedProfileState) valid() bool {
	return state.generation != [32]byte{} && state.epoch != 0 && state.accepted != [32]byte{} &&
		(state.conflict == [32]byte{} || state.conflict != state.accepted)
}

func encodeClosedProfileState(state closedProfileState) []byte {
	raw := make([]byte, 0, closedProfileStateSize)
	raw = append(raw, "ARDCPST1"...)
	raw = append(raw, state.generation[:]...)
	raw = binary.BigEndian.AppendUint64(raw, state.epoch)
	raw = append(raw, state.accepted[:]...)
	return append(raw, state.conflict[:]...)
}

func decodeClosedProfileState(raw []byte) (closedProfileState, error) {
	if len(raw) != closedProfileStateSize || string(raw[:8]) != "ARDCPST1" {
		return closedProfileState{}, errors.New("closed profile state framing is invalid")
	}
	state := closedProfileState{}
	copy(state.generation[:], raw[8:40])
	state.epoch = binary.BigEndian.Uint64(raw[40:48])
	copy(state.accepted[:], raw[48:80])
	copy(state.conflict[:], raw[80:112])
	if !state.valid() {
		return closedProfileState{}, errors.New("closed profile state is invalid")
	}
	return state, nil
}

func closedProfileStateName(generation [32]byte) string {
	return fmt.Sprintf("closed-profile-state-%x", generation)
}

func closedProfileBytesName(generation [32]byte) string {
	return fmt.Sprintf("closed-profile-%x.bin", generation)
}

func (root *durableRoot) loadClosedProfile(generation [32]byte) (closedProfileState, []byte, error) {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil {
		return closedProfileState{}, nil, err
	}
	stateName, profileName := closedProfileStateName(generation), closedProfileBytesName(generation)
	stateRaw, err := readBoundedFile(filepath.Join(root.path, stateName), closedProfileStateSize)
	if os.IsNotExist(err) {
		if _, statErr := os.Lstat(filepath.Join(root.path, profileName)); statErr == nil {
			return closedProfileState{}, nil, errors.New("closed profile bytes lack durable state")
		} else if !os.IsNotExist(statErr) {
			return closedProfileState{}, nil, statErr
		}
		return closedProfileState{}, nil, nil
	}
	if err != nil {
		return closedProfileState{}, nil, fmt.Errorf("read closed profile state: %w", err)
	}
	state, err := decodeClosedProfileState(stateRaw)
	if err != nil {
		return closedProfileState{}, nil, err
	}
	if state.generation != generation {
		return closedProfileState{}, nil, errors.New("closed profile state generation is wrong")
	}
	profile, err := readBoundedFile(filepath.Join(root.path, profileName), maximumClosedProfileSize)
	if err != nil || sha256.Sum256(profile) != state.accepted {
		return closedProfileState{}, nil, errors.New("closed profile bytes disagree with durable state")
	}
	return state, profile, nil
}

func (root *durableRoot) commitClosedProfile(state closedProfileState, profile []byte) error {
	root.mu.Lock()
	defer root.mu.Unlock()
	if err := root.available(); err != nil || !state.valid() || len(profile) == 0 || len(profile) > maximumClosedProfileSize || sha256.Sum256(profile) != state.accepted {
		return errors.New("closed profile durable input is invalid")
	}
	profilePath := filepath.Join(root.path, closedProfileBytesName(state.generation))
	if existing, err := readBoundedFile(profilePath, maximumClosedProfileSize); err == nil {
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
	return replaceClosedProfileState(root.path, closedProfileStateName(state.generation), encodeClosedProfileState(state))
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
