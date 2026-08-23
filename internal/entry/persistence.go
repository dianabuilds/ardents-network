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
	if state.Version != 1 || state.Generation == 0 || len(state.Records) > 4 || len(state.Contacts) > 4 || len(state.Admissions) > maximumAdmissions || state.Previous != "" && !stateName.MatchString(state.Previous) {
		return false
	}
	seen, active, retained := map[[32]byte]bool{}, [2]bool{}, [2]byte{}
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
			retained[record.Slot]++
		case memberDraining, memberVerified:
			if len(record.Invite) == 0 || len(record.Invite) > maximumInviteSize || retained[record.Slot] >= 2 {
				return false
			}
			retained[record.Slot]++
		case memberRetired:
			if len(record.Invite) != 0 {
				return false
			}
		default:
			return false
		}
	}
	return validAttemptState(state) && validAdmissions(state.Admissions)
}

func validAdmissions(records []admissionRecord) bool {
	seen := map[[96]byte]bool{}
	for _, record := range records {
		var key [96]byte
		copy(key[:32], record.InviteID[:])
		copy(key[32:64], record.AttachmentID[:])
		copy(key[64:], record.ClientKeyDigest[:])
		if record.InviteID == [32]byte{} || record.AttachmentID == [32]byte{} || record.ClientKeyDigest == [32]byte{} ||
			record.NotAfter <= 0 || seen[key] {
			return false
		}
		seen[key] = true
	}
	return true
}

func validAttemptState(state durableState) bool {
	if state.Attempt == nil {
		return len(state.Contacts) == 0
	}
	attempt := state.Attempt
	if attempt.ID == [32]byte{} || attempt.Started <= 0 || attempt.Deadline <= attempt.Started ||
		attempt.Terminal == "" && attempt.Ended != 0 || attempt.Terminal != "" && (attempt.Ended < attempt.Started || !validAttemptTerminal(attempt.Terminal)) {
		return false
	}
	previous := -1
	for _, contact := range state.Contacts {
		record, found := state.find(contact.InviteID)
		if contact.AttemptID != attempt.ID || contact.InviteID == [32]byte{} || contact.Slot > 1 || contact.Ordinal > 3 ||
			contact.Slot != contact.Ordinal/2 || int(contact.Ordinal) <= previous || contact.Started < attempt.Started ||
			!found || record.Slot != contact.Slot || contact.Outcome == "" && contact.Terminal != 0 ||
			contact.Outcome != "" && (contact.Terminal < contact.Started || !validContactOutcome(contact.Outcome)) {
			return false
		}
		previous = int(contact.Ordinal)
	}
	return true
}

func validAttemptTerminal(value string) bool {
	switch value {
	case "", "opened", "entry-attempt-exhausted", "entry-deadline-exceeded", "entry-interrupted", "entry-local-denial":
		return true
	default:
		return false
	}
}

func validContactOutcome(value string) bool {
	return value == "opened" || value == "failed" || value == "interrupted"
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
