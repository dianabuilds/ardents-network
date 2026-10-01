package duty

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
	"strconv"
	"strings"
)

var stateName = regexp.MustCompile(`^[0-9a-f]{64}$`)

func loadState(root string) (durableState, string, error) {
	watermarkGeneration, watermarkName, hasWatermark, err := loadWatermark(root)
	if err != nil {
		return durableState{}, "", err
	}
	pointer, err := readBounded(filepath.Join(root, "current"), 65)
	if os.IsNotExist(err) {
		if !hasWatermark {
			return durableState{Version: durableStateVersion, Duties: []dutyRecord{}}, "", nil
		}
		state, loadErr := loadGeneration(root, watermarkName)
		if loadErr != nil || state.Generation != watermarkGeneration {
			return durableState{}, "", errors.New("local role watermark target is invalid")
		}
		if err := replaceFile(root, "current", "current", watermarkName+"\n"); err != nil {
			return durableState{}, "", err
		}
		return state, watermarkName, nil
	}
	if err != nil {
		return durableState{}, "", err
	}
	current := strings.TrimSuffix(string(pointer), "\n")
	if string(pointer) != current+"\n" || !stateName.MatchString(current) {
		return durableState{}, "", errors.New("local role state pointer is invalid")
	}
	state, err := loadGeneration(root, current)
	if err != nil {
		return durableState{}, "", err
	}
	if !hasWatermark || state.Generation > watermarkGeneration ||
		state.Generation == watermarkGeneration && current != watermarkName {
		return durableState{}, "", errors.New("local role state violates its watermark")
	}
	if state.Generation < watermarkGeneration {
		state, err = loadGeneration(root, watermarkName)
		if err != nil || state.Generation != watermarkGeneration {
			return durableState{}, "", errors.New("local role watermark target is invalid")
		}
		current = watermarkName
		if err := replaceFile(root, "current", "current", current+"\n"); err != nil {
			return durableState{}, "", err
		}
	}
	return state, current, nil
}

func loadGeneration(root, name string) (durableState, error) {
	raw, err := readBounded(filepath.Join(root, "state-"+name), maximumStateBytes)
	if err != nil || sha256Hex(raw) != name {
		return durableState{}, errors.New("local role generation is invalid")
	}
	var envelope struct {
		Version uint8 `json:"version"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return durableState{}, errors.New("local role generation is invalid")
	}
	switch envelope.Version {
	case durableStateVersion:
		var state durableState
		if err := strictDecodeGeneration(raw, &state); err != nil || !validDurableState(state) {
			return durableState{}, errors.New("local role generation is invalid")
		}
		return state, nil
	case legacyDurableStateVersion:
		var legacy legacyDurableState
		if err := strictDecodeGeneration(raw, &legacy); err != nil || !validLegacyDurableState(legacy) {
			return durableState{}, errors.New("local role generation is invalid")
		}
		return legacy.convert(), nil
	default:
		return durableState{}, errors.New("local role generation is invalid")
	}
}

func strictDecodeGeneration(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("local role generation has trailing data")
	}
	return nil
}

func loadWatermark(root string) (uint64, string, bool, error) {
	raw, err := readBounded(filepath.Join(root, "watermark"), 96)
	if os.IsNotExist(err) {
		return 0, "", false, nil
	}
	if err != nil {
		return 0, "", false, err
	}
	line := strings.TrimSuffix(string(raw), "\n")
	parts := strings.Split(line, " ")
	if len(parts) != 2 || string(raw) != line+"\n" || !stateName.MatchString(parts[1]) {
		return 0, "", false, errors.New("local role watermark is invalid")
	}
	generation, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil || generation == 0 || fmt.Sprintf("%d %s\n", generation, parts[1]) != string(raw) {
		return 0, "", false, errors.New("local role watermark is invalid")
	}
	return generation, parts[1], true, nil
}

func (store *store) commit(next durableState) error {
	next.Version = durableStateVersion
	next.Generation = store.state.Generation + 1
	next.Previous = store.current
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > maximumStateBytes {
		return errors.New("local role state exceeds its byte bound")
	}
	name := sha256Hex(raw)
	if err := writeGeneration(store.root, name, raw); err != nil {
		store.failed = err
		return err
	}
	if err := replaceFile(store.root, "watermark", "watermark", fmt.Sprintf("%d %s\n", next.Generation, name)); err != nil {
		store.failed = err
		return err
	}
	if err := replaceFile(store.root, "current", "current", name+"\n"); err != nil {
		store.failed = err
		return err
	}
	if err := cleanupGenerations(store.root, name, store.current); err != nil {
		store.failed = err
		return err
	}
	store.state, store.current = next, name
	return nil
}

func validDurableState(state durableState) bool {
	return state.Version == durableStateVersion && state.Generation > 0 &&
		(state.Previous == "" || stateName.MatchString(state.Previous)) && validRecords(state.Duties)
}

// legacyDurableStateVersion is the retired version-1 schema. ADR-0107 keeps
// its strict decoder solely for the bounded in-place conversion: a legacy
// generation stays a valid watermark/rollback target, its conflict duties and
// generation continuity survive, and its Transit Grant spend records - which
// no writer emits and no reader consumes since ADR-0093 retired the spend
// operation - are dropped.
const legacyDurableStateVersion = uint8(1)

// legacyDurableState is the exact retired version-1 generation shape.
type legacyDurableState struct {
	Version            uint8               `json:"version"`
	Generation         uint64              `json:"generation"`
	Previous           string              `json:"previous,omitempty"`
	Duties             []dutyRecord        `json:"duties"`
	TransitGrantSpends []transitGrantSpend `json:"transit_grant_spends"`
}

// transitGrantSpend is one historical Node-local, finite, irreversible
// consumption of an already State-authorized transit admission capability.
// It has no Target, Service, or client material.
type transitGrantSpend struct {
	NodeID   [32]byte `json:"node_id"`
	GrantID  [32]byte `json:"grant_id"`
	NotAfter int64    `json:"not_after"`
}

const maximumTransitGrantSpends = 64

func (legacy legacyDurableState) convert() durableState {
	return durableState{Version: durableStateVersion, Generation: legacy.Generation,
		Previous: legacy.Previous, Duties: legacy.Duties}
}

func validLegacyDurableState(state legacyDurableState) bool {
	return state.Version == legacyDurableStateVersion && state.Generation > 0 &&
		(state.Previous == "" || stateName.MatchString(state.Previous)) && validRecords(state.Duties) &&
		validTransitGrantSpends(state.TransitGrantSpends)
}

func validRecords(records []dutyRecord) bool {
	return validateRecords(records) == nil
}

func validateRecords(records []dutyRecord) error {
	if len(records) > 64 {
		return ErrLocalRoleRecordLimit
	}
	producers := map[[32]byte]bool{}
	seen := map[[3][32]byte]bool{}
	for _, record := range records {
		if record.Producer == ([32]byte{}) || record.Identity == ([32]byte{}) || record.Family == ([32]byte{}) ||
			!validClass(record.Class) || !validState(record.State) || record.NotAfter <= 0 {
			return errors.New("local role record is invalid")
		}
		producers[record.Producer] = true
		key := [3][32]byte{record.Producer, record.Identity, record.Family}
		if seen[key] {
			return errors.New("local role record is duplicated")
		}
		seen[key] = true
	}
	if len(producers) > 16 {
		return ErrLocalRoleProducerLimit
	}
	if !conflictFree(records) {
		return ErrLocalRoleConflict
	}
	return nil
}

func validTransitGrantSpends(spends []transitGrantSpend) bool {
	if len(spends) > maximumTransitGrantSpends {
		return false
	}
	seen := map[[32]byte]bool{}
	for _, spend := range spends {
		if spend.NodeID == [32]byte{} || spend.GrantID == [32]byte{} || spend.NotAfter <= 0 || seen[spend.GrantID] {
			return false
		}
		seen[spend.GrantID] = true
	}
	return true
}

func conflictFree(records []dutyRecord) bool {
	for first := range records {
		if records[first].Class == "ordinary-initiator" {
			continue
		}
		for second := first + 1; second < len(records); second++ {
			if records[second].Class != "ordinary-initiator" &&
				(records[first].Identity == records[second].Identity || records[first].Family == records[second].Family) {
				if records[first].Producer == records[second].Producer &&
					records[first].Class == "direct-source" && records[second].Class == "direct-source" {
					continue
				}
				return false
			}
		}
	}
	return true
}

func sha256Hex(raw []byte) string {
	digest := sha256.Sum256(raw)
	return hex.EncodeToString(digest[:])
}
