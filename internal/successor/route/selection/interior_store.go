//go:build linux

package selection

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/route"
)

const interiorMarker = ".ardents-interior-set-v1"

type interiorRecord struct {
	Version    uint8             `json:"version"`
	Network    [32]byte          `json:"network"`
	Domain     uint8             `json:"domain"`
	Generation uint64            `json:"generation"`
	Previous   string            `json:"previous,omitempty"`
	Entries    [2]route.Member   `json:"entries"`
	Set        route.InteriorSet `json:"set"`
}
type interiorStore struct {
	root, name string
	lease      rootLease
	record     interiorRecord
	failure    error
}

func openInteriorStore(root string, network [32]byte, domain uint8) (_ *interiorStore, result error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	root = absolute
	if err := inspectRoot(root); err != nil {
		return nil, err
	}
	if err := validateRootPermissions(root); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	fresh := len(entries) == 0
	lease, err := acquireRootLease(root)
	if err != nil {
		return nil, err
	}
	s := &interiorStore{root: root, lease: lease, record: interiorRecord{Version: 1, Network: network, Domain: domain}}
	defer func() {
		if result != nil {
			result = errors.Join(result, lease.release())
		}
	}()
	if fresh {
		entries, err = os.ReadDir(root)
		if err != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
			return nil, errors.New("interior root changed")
		}
		if err := writeExclusive(filepath.Join(root, interiorMarker), []byte("ardents-interior-set-v1\n")); err != nil {
			return nil, err
		}
		if err := s.commit(s.record); err != nil {
			return nil, err
		}
	} else {
		if err := inspectInteriorRoot(root); err != nil {
			return nil, err
		}
		floor, exists, err := loadWatermark(root)
		if err != nil || !exists {
			return nil, errors.New("interior floor unavailable")
		}
		record, err := readInteriorRecord(root, floor.name)
		if err != nil || record.Generation != floor.generation || record.Network != network || record.Domain != domain {
			return nil, errors.New("interior retained binding unavailable")
		}
		pointer, err := readBounded(filepath.Join(root, "current"), 65)
		name := strings.TrimSuffix(string(pointer), "\n")
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if err == nil {
			if string(pointer) != name+"\n" {
				return nil, errors.New("interior pointer invalid")
			}
			prior, err := readInteriorRecord(root, name)
			if err != nil || prior.Generation > floor.generation || prior.Generation == floor.generation && name != floor.name {
				return nil, errors.New("interior pointer violates floor")
			}
		}
		if record.Previous != "" {
			prior, err := readInteriorRecord(root, record.Previous)
			if err != nil || prior.Generation+1 != record.Generation || prior.Network != network || prior.Domain != domain {
				return nil, errors.New("interior predecessor unavailable")
			}
		}
		if name != floor.name {
			if err := replaceCurrent(root, floor.name); err != nil {
				return nil, err
			}
		}
		s.record, s.name = record, floor.name
		if err := cleanupGenerations(root, s.name, record.Previous); err != nil {
			return nil, err
		}
	}
	if err := cleanupClosedSetStaging(root); err != nil {
		return nil, err
	}
	return s, nil
}
func inspectInteriorRoot(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) > 10 {
		return errors.New("interior root bound exceeded")
	}
	for _, entry := range entries {
		name := entry.Name()
		allowed := name == interiorMarker || name == rootLockName || name == "current" || name == "watermark" || strings.HasPrefix(name, "state-") || strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-")
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !allowed || !info.Mode().IsRegular() {
			return errors.New("interior root entry unavailable")
		}
	}
	marker, err := readBounded(filepath.Join(root, interiorMarker), 64)
	if err != nil || string(marker) != "ardents-interior-set-v1\n" {
		return errors.New("interior marker unavailable")
	}
	return nil
}
func readInteriorRecord(root, name string) (interiorRecord, error) {
	if !stateName.MatchString(name) {
		return interiorRecord{}, errors.New("interior generation invalid")
	}
	raw, err := readBounded(filepath.Join(root, "state-"+name), maximumStateBytes)
	if err != nil || sha256Hex(raw) != name {
		return interiorRecord{}, errors.New("interior generation unavailable")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var record interiorRecord
	if err := decoder.Decode(&record); err != nil {
		return interiorRecord{}, err
	}
	if decoder.Decode(&struct{}{}) != io.EOF || !validInteriorRecord(record) {
		return interiorRecord{}, errors.New("interior record invalid")
	}
	return record, nil
}
func validInteriorRecord(record interiorRecord) bool {
	if record.Version != 1 || record.Network == [32]byte{} || closedAdjacentIndex(record.Domain) < 0 || record.Generation == 0 ||
		record.Generation == 1 && record.Previous != "" || record.Generation > 1 && !stateName.MatchString(record.Previous) {
		return false
	}
	if record.Set == (route.InteriorSet{}) {
		return record.Entries == [2]route.Member{}
	}
	candidates := []route.Member{record.Set.Members[0], record.Set.Members[1]}
	for _, member := range append(candidates, record.Entries[:]...) {
		if !validClosedSetMember(ClosedSetMember(member)) || member.Domain != record.Domain {
			return false
		}
	}
	return record.Set.Check(candidates, record.Entries, record.Set.Chosen, record.Domain) == nil
}
func (s *interiorStore) commit(next interiorRecord) error {
	if s.record.Generation == ^uint64(0) {
		return errors.New("interior generation exhausted")
	}
	next.Generation = s.record.Generation + 1
	next.Previous = s.name
	if !validInteriorRecord(next) {
		return errors.New("interior commit invalid")
	}
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > maximumStateBytes {
		return errors.New("interior record exceeds bound")
	}
	name := sha256Hex(raw)
	if err := writeGeneration(s.root, filepath.Join(s.root, "state-"+name), raw); err != nil {
		return err
	}
	if _, err := readInteriorRecord(s.root, name); err != nil {
		return err
	}
	if err := replaceWatermark(s.root, next.Generation, name); err != nil {
		return err
	}
	if err := replaceCurrent(s.root, name); err != nil {
		return err
	}
	if err := cleanupGenerations(s.root, name, next.Previous); err != nil {
		return err
	}
	s.record, s.name = next, name
	return nil
}
func (s *interiorStore) selectPair(candidates []route.Member, entries [2]route.Member, now time.Time) (route.InteriorSet, error) {
	if s.failure != nil {
		return route.InteriorSet{}, s.failure
	}
	retained := s.record.Set
	if !retained.Chosen.IsZero() && now.Before(retained.Chosen) {
		return route.InteriorSet{}, errors.New("interior time regressed")
	}
	if !retained.Chosen.IsZero() && now.Before(retained.NotAfter) {
		if s.record.Entries != entries {
			return route.InteriorSet{}, errors.New("entry pair changed inside retained Interior horizon")
		}
		return retained, retained.Check(candidates, entries, now, s.record.Domain)
	}
	selected, err := route.SelectInterior(candidates, entries, now, s.record.Domain)
	if err != nil {
		return route.InteriorSet{}, err
	}
	next := s.record
	next.Set = selected
	next.Entries = entries
	if err := s.commit(next); err != nil {
		s.failure = err
		return route.InteriorSet{}, err
	}
	return selected, nil
}
func (s *interiorStore) close() error {
	s.failure = errors.Join(s.failure, s.lease.release())
	return s.failure
}
