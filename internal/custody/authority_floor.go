package custody

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type floorDocument struct {
	Profile       string           `json:"profile"`
	SchemaVersion uint64           `json:"schema_version"`
	Floors        []authorityFloor `json:"floors"`
}

type authorityFloor struct {
	Environment  string        `json:"environment"`
	Network      string        `json:"network"`
	Root         string        `json:"root"`
	Kind         AuthorityKind `json:"kind"`
	IDCommitment string        `json:"id_commitment"`
	Generation   uint64        `json:"generation"`
	Revision     uint64        `json:"revision"`
	Watermarks   []Watermark   `json:"watermarks"`
}

func (vault *Vault) prepareFloor(state AuthorityState) error {
	if err := validateAuthorityState(state); err != nil {
		return err
	}
	floors, err := vault.readFloors()
	if err != nil {
		return err
	}
	if previous, found := floorFor(floors, state.Binding); found && !strictlyHigher(state, previous) {
		return ErrInvalid
	}
	return nil
}

func (vault *Vault) advanceFloor(state AuthorityState) error {
	floors, err := vault.readFloors()
	if err != nil {
		return err
	}
	if previous, found := floorFor(floors, state.Binding); found && !strictlyHigher(state, previous) {
		return ErrInvalid
	}
	next := floorFromState(state)
	replaced := false
	for index := range floors {
		if floorBinding(floors[index]) == state.Binding {
			floors[index] = next
			replaced = true
			break
		}
	}
	if !replaced {
		floors = append(floors, next)
	}
	sort.Slice(floors, func(i, j int) bool { return floorKey(floors[i]) < floorKey(floors[j]) })
	return vault.writeFloors(floors)
}

func (vault *Vault) matchesFloor(state AuthorityState) error {
	floors, err := vault.readFloors()
	if err != nil {
		return err
	}
	floor, found := floorFor(floors, state.Binding)
	if !found || floor.Generation != state.Generation || floor.Revision != state.Revision || !equalWatermarks(floor.Watermarks, state.Watermarks) {
		return ErrInvalid
	}
	return nil
}

func (vault *Vault) readFloors() ([]authorityFloor, error) {
	raw, err := readSmallFile(vault.floors)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read authority floors: %w", err)
	}
	defer zero(raw)
	var document floorDocument
	if err := decodeCanonical(raw, &document, maximumFloorBytes); err != nil {
		return nil, ErrInvalid
	}
	if document.Profile != "ardents-authority-floor-v1" || document.SchemaVersion != 1 || len(document.Floors) > maximumVaultRecords {
		return nil, ErrInvalid
	}
	for index, floor := range document.Floors {
		if err := validateFloor(floor); err != nil {
			return nil, err
		}
		if index > 0 && floorKey(document.Floors[index-1]) >= floorKey(floor) {
			return nil, ErrInvalid
		}
	}
	return document.Floors, nil
}

func (vault *Vault) writeFloors(floors []authorityFloor) error {
	raw, err := marshalCanonical(floorDocument{Profile: "ardents-authority-floor-v1", SchemaVersion: 1, Floors: floors})
	if err != nil {
		return err
	}
	defer zero(raw)
	if err := writeAtomicPrivate(vault.floors, raw); err != nil {
		return err
	}
	persisted, err := vault.readFloors()
	if err != nil {
		return err
	}
	if !equalFloors(persisted, floors) {
		return ErrInvalid
	}
	return nil
}

func floorFromState(state AuthorityState) authorityFloor {
	return authorityFloor{
		Environment: hex.EncodeToString(state.Binding.Environment[:]),
		Network:     hex.EncodeToString(state.Binding.Network[:]),
		Root:        hex.EncodeToString(state.Binding.Root[:]),
		Kind:        state.Binding.Kind, IDCommitment: hex.EncodeToString(state.Binding.IDCommitment[:]),
		Generation: state.Generation, Revision: state.Revision, Watermarks: append([]Watermark(nil), state.Watermarks...),
	}
}

func floorBinding(floor authorityFloor) AuthorityBinding {
	var binding AuthorityBinding
	for _, value := range []struct {
		encoded string
		dest    []byte
	}{
		{floor.Environment, binding.Environment[:]},
		{floor.Network, binding.Network[:]},
		{floor.Root, binding.Root[:]},
		{floor.IDCommitment, binding.IDCommitment[:]},
	} {
		decoded, _ := hex.DecodeString(value.encoded)
		copy(value.dest, decoded)
	}
	binding.Kind = floor.Kind
	return binding
}

func floorFor(floors []authorityFloor, binding AuthorityBinding) (authorityFloor, bool) {
	for _, floor := range floors {
		if floorBinding(floor) == binding {
			return floor, true
		}
	}
	return authorityFloor{}, false
}

func floorKey(floor authorityFloor) string {
	return floor.Environment + floor.Network + floor.Root + string(floor.Kind) + floor.IDCommitment
}

func validateFloor(floor authorityFloor) error {
	for _, encoded := range []string{floor.Environment, floor.Network, floor.Root, floor.IDCommitment} {
		decoded, err := hex.DecodeString(encoded)
		if len(encoded) != 64 || err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != encoded {
			return ErrInvalid
		}
	}
	if (floor.Kind != AuthorityService && floor.Kind != AuthorityName) || !validWatermarks(floor.Watermarks) {
		return ErrInvalid
	}
	return nil
}

func strictlyHigher(state AuthorityState, floor authorityFloor) bool {
	if state.Generation <= floor.Generation || state.Revision <= floor.Revision {
		return false
	}
	for _, previous := range floor.Watermarks {
		index := sort.Search(len(state.Watermarks), func(index int) bool { return state.Watermarks[index].Domain >= previous.Domain })
		if index == len(state.Watermarks) || state.Watermarks[index].Domain != previous.Domain || state.Watermarks[index].Value <= previous.Value {
			return false
		}
	}
	return true
}

func equalWatermarks(left, right []Watermark) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func equalFloors(left, right []authorityFloor) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Environment != right[index].Environment || left[index].Network != right[index].Network ||
			left[index].Root != right[index].Root || left[index].Kind != right[index].Kind ||
			left[index].IDCommitment != right[index].IDCommitment || left[index].Generation != right[index].Generation ||
			left[index].Revision != right[index].Revision || !equalWatermarks(left[index].Watermarks, right[index].Watermarks) {
			return false
		}
	}
	return true
}

func validWatermarks(watermarks []Watermark) bool {
	if len(watermarks) == 0 || len(watermarks) > maximumWatermarks {
		return false
	}
	for index, watermark := range watermarks {
		if len(watermark.Domain) == 0 || len(watermark.Domain) > maximumWatermarkDomainBytes || !isASCII(watermark.Domain) {
			return false
		}
		if index > 0 && watermarks[index-1].Domain >= watermark.Domain {
			return false
		}
	}
	return true
}

func writeAtomicPrivate(path string, body []byte) error {
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".authority-floor-")
	if err != nil {
		return fmt.Errorf("create authority floor temporary: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("protect authority floor temporary: %w", err)
	}
	if _, err := temporary.Write(body); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write authority floor: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("flush authority floor: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close authority floor: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("publish authority floor: %w", err)
	}
	return nil
}
