package entry

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const maximumStateBytes = 64 << 10

var stateName = regexp.MustCompile(`^[0-9a-f]{64}$`)

func loadState(root string) (durableState, string, error) {
	watermark, hasWatermark, err := loadWatermark(root)
	if err != nil {
		return durableState{}, "", err
	}
	pointer, err := readBounded(filepath.Join(root, "current"), 65)
	if os.IsNotExist(err) {
		if !hasWatermark {
			return durableState{Version: 1, Records: []memberRecord{}}, "", nil
		}
		state, loadErr := loadGeneration(root, watermark.name)
		if loadErr != nil || state.Generation != watermark.generation {
			return durableState{}, "", errors.New("entry generation watermark target is invalid")
		}
		if err := replaceCurrent(root, watermark.name); err != nil {
			return durableState{}, "", err
		}
		return state, watermark.name, verifyPrevious(root, state)
	}
	if err != nil {
		return durableState{}, "", err
	}
	current := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != current+"\n" || !stateName.MatchString(current) {
		return durableState{}, "", errors.New("entry state pointer is invalid")
	}
	state, err := loadGeneration(root, current)
	if err != nil {
		return durableState{}, "", err
	}
	if !hasWatermark || state.Generation > watermark.generation ||
		state.Generation == watermark.generation && current != watermark.name {
		return durableState{}, "", errors.New("entry state generation violates its watermark")
	}
	if state.Generation < watermark.generation {
		state, err = loadGeneration(root, watermark.name)
		if err != nil || state.Generation != watermark.generation {
			return durableState{}, "", errors.New("entry generation watermark target is invalid")
		}
		current = watermark.name
		if err := replaceCurrent(root, current); err != nil {
			return durableState{}, "", err
		}
	}
	return state, current, verifyPrevious(root, state)
}

func loadGeneration(root, current string) (durableState, error) {
	raw, err := readBounded(filepath.Join(root, "state-"+current), maximumStateBytes)
	if err != nil || sha256Hex(raw) != current {
		return durableState{}, errors.New("entry state generation is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var state durableState
	if err := decoder.Decode(&state); err != nil {
		return durableState{}, errors.New("entry state generation is invalid")
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || !validState(state) {
		return durableState{}, errors.New("entry state generation is invalid")
	}
	return state, nil
}

func verifyPrevious(root string, state durableState) error {
	if state.Previous == "" {
		return nil
	}
	previous, err := loadGeneration(root, state.Previous)
	if err != nil || previous.Generation+1 != state.Generation {
		return errors.New("entry state predecessor is invalid")
	}
	return nil
}

func validState(state durableState) bool {
	if state.Version != 1 || state.Generation == 0 || len(state.Records) > 4 || state.Previous != "" && !stateName.MatchString(state.Previous) {
		return false
	}
	seen, active := map[[32]byte]bool{}, [2]bool{}
	for _, record := range state.Records {
		if seen[record.InviteID] || record.InviteID == [32]byte{} || record.Identity == [32]byte{} || record.Family == [32]byte{} ||
			record.Slot > 1 || record.Generation < 1 || record.Generation > 2 {
			return false
		}
		seen[record.InviteID] = true
		switch record.Status {
		case memberActive:
			if active[record.Slot] || len(record.Invite) == 0 || len(record.Invite) > maximumInviteSize {
				return false
			}
			active[record.Slot] = true
		case memberRetired:
			if len(record.Invite) != 0 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

func (owner *owner) commit(next durableState, retainPrevious bool) error {
	next.Version = 1
	next.Generation = owner.state.Generation + 1
	next.Previous = ""
	if retainPrevious && owner.current != "" {
		next.Previous = owner.current
	}
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > maximumStateBytes {
		return errors.New("entry state exceeds its bound")
	}
	name := sha256Hex(raw)
	final := filepath.Join(owner.root, "state-"+name)
	if existing, readErr := readBounded(final, maximumStateBytes); readErr == nil {
		if !bytes.Equal(existing, raw) {
			return errors.New("immutable entry generation disagrees")
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	} else if err := writeGeneration(owner.root, final, raw); err != nil {
		return err
	}
	if err := replaceWatermark(owner.root, next.Generation, name); err != nil {
		return err
	}
	if err := replaceCurrent(owner.root, name); err != nil {
		return err
	}
	if !retainPrevious {
		if err := cleanupGenerations(owner.root, name, ""); err != nil {
			return err
		}
	}
	owner.state, owner.current = next, name
	return nil
}

func sha256Hex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}

func readBounded(path string, maximum int64) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(file, maximum+1))
	closeErr := file.Close()
	if readErr != nil {
		return nil, readErr
	}
	if closeErr != nil {
		return nil, closeErr
	}
	if int64(len(raw)) > maximum {
		return nil, fmt.Errorf("bounded file exceeds %d bytes", maximum)
	}
	return raw, nil
}
