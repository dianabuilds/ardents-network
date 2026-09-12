//go:build linux

package entry

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const closedSetMarker = ".ardents-entry-set-v1"

// OpenClosedSets opens a fresh or previously committed closed Entry selection
// owner. Initial creation and subsequent commits precede every returned set;
// ambiguous interrupted creation refuses instead of drawing replacement peers.
func OpenClosedSets(config ClosedSetConfig) (*ClosedSets, error) {
	if config.Root == "" || config.NetworkID == [32]byte{} || config.Current == nil {
		return nil, errors.New("closed Entry setup incomplete")
	}
	root, err := filepath.Abs(config.Root)
	if err != nil {
		return nil, err
	}
	if err := inspectRoot(root); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	fresh := len(entries) == 0
	if !fresh {
		if err := inspectClosedSetRoot(root); err != nil {
			return nil, err
		}
	}
	if err := validateRootPermissions(root); err != nil {
		return nil, err
	}
	lease, err := acquireRootLease(root)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			_ = lease.release()
		}
	}()
	owner := &ClosedSets{root: root, lease: lease, current: config.Current, state: closedSetState{Version: 1, NetworkID: config.NetworkID}}
	if fresh {
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 || entries[0].Name() != rootLockName {
			return nil, errors.New("closed Entry root changed before claim")
		}
		if err := writeExclusive(filepath.Join(root, closedSetMarker), []byte("ardents-entry-set-v1\n")); err != nil {
			return nil, err
		}
		if err := owner.commitClosedSet(owner.state); err != nil {
			return nil, err
		}
	} else {
		if err := inspectClosedSetRoot(root); err != nil {
			return nil, err
		}
		retained, name, err := loadClosedSets(root)
		if err != nil || retained.NetworkID != config.NetworkID {
			return nil, errors.New("closed Entry retained authority unavailable")
		}
		owner.state, owner.name = retained, name
		if err := cleanupGenerations(root, name, retained.Previous); err != nil {
			return nil, err
		}
	}
	if err := cleanupClosedSetStaging(root); err != nil {
		return nil, err
	}
	opened = true
	return owner, nil
}

func inspectClosedSetRoot(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) > 10 {
		return errors.New("closed Entry root bound exceeded")
	}
	for _, entry := range entries {
		name := entry.Name()
		allowed := name == closedSetMarker || name == rootLockName || name == "current" || name == "watermark" ||
			strings.HasPrefix(name, "state-") || strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-")
		info, err := os.Lstat(filepath.Join(root, name))
		if err != nil || !allowed || !info.Mode().IsRegular() {
			return errors.New("closed Entry root entry unavailable")
		}
	}
	marker, err := readBounded(filepath.Join(root, closedSetMarker), 64)
	if err != nil || string(marker) != "ardents-entry-set-v1\n" {
		return errors.New("closed Entry root marker unavailable")
	}
	return nil
}

func loadClosedSets(root string) (closedSetState, string, error) {
	floor, exists, err := loadWatermark(root)
	if err != nil || !exists {
		return closedSetState{}, "", errors.New("closed Entry floor unavailable")
	}
	current, err := readBounded(filepath.Join(root, "current"), 65)
	if err != nil && !os.IsNotExist(err) {
		return closedSetState{}, "", err
	}
	name := strings.TrimSuffix(string(current), "\n")
	if err == nil {
		if string(current) != name+"\n" || !stateName.MatchString(name) {
			return closedSetState{}, "", errors.New("closed Entry pointer invalid")
		}
		pointed, err := readClosedSetGeneration(root, name)
		if err != nil || pointed.Generation > floor.generation || pointed.Generation == floor.generation && name != floor.name {
			return closedSetState{}, "", errors.New("closed Entry pointer violates floor")
		}
	}
	retained, err := readClosedSetGeneration(root, floor.name)
	if err != nil || retained.Generation != floor.generation {
		return closedSetState{}, "", errors.New("closed Entry floor target unavailable")
	}
	if retained.Previous != "" {
		prior, err := readClosedSetGeneration(root, retained.Previous)
		if err != nil || prior.Generation+1 != retained.Generation || prior.NetworkID != retained.NetworkID {
			return closedSetState{}, "", errors.New("closed Entry predecessor unavailable")
		}
	}
	if name != floor.name {
		if err := replaceCurrent(root, floor.name); err != nil {
			return closedSetState{}, "", err
		}
	}
	return retained, floor.name, nil
}

func readClosedSetGeneration(root, name string) (closedSetState, error) {
	if !stateName.MatchString(name) {
		return closedSetState{}, errors.New("closed Entry generation name invalid")
	}
	raw, err := readBounded(filepath.Join(root, "state-"+name), maximumStateBytes)
	if err != nil || sha256Hex(raw) != name {
		return closedSetState{}, errors.New("closed Entry generation invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var retained closedSetState
	if err := decoder.Decode(&retained); err != nil {
		return closedSetState{}, err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) || !validClosedSetState(retained) {
		return closedSetState{}, errors.New("closed Entry state invalid")
	}
	return retained, nil
}

func validClosedSetState(retained closedSetState) bool {
	if retained.Version != 1 || retained.NetworkID == [32]byte{} || retained.Generation == 0 ||
		retained.Generation == 1 && retained.Previous != "" || retained.Generation > 1 && !stateName.MatchString(retained.Previous) {
		return false
	}
	for index, selected := range retained.Sets {
		if selected == (closedDomainSet{}) {
			continue
		}
		if selected.Chosen.IsZero() || !selected.Chosen.Before(selected.NotAfter) || selected.NotAfter.After(selected.Chosen.Add(6*time.Hour)) {
			return false
		}
		for _, member := range selected.Members {
			if !validClosedSetMember(member) || closedAdjacentIndex(member.Domain) != index || selected.NotAfter.After(member.NotAfter) {
				return false
			}
		}
		first, second := selected.Members[0], selected.Members[1]
		if first.NodeID == second.NodeID || first.PublicKey == second.PublicKey || first.FamilyID == second.FamilyID {
			return false
		}
	}
	return true
}

func (owner *ClosedSets) commitClosedSet(next closedSetState) error {
	if next.Generation == ^uint64(0) {
		return errors.New("closed Entry generation exhausted")
	}
	next.Generation = owner.state.Generation + 1
	next.Previous = owner.name
	if !validClosedSetState(next) {
		return errors.New("closed Entry commit invalid")
	}
	raw, err := json.Marshal(next)
	if err != nil || len(raw) > maximumStateBytes {
		return errors.New("closed Entry state bound exceeded")
	}
	name := sha256Hex(raw)
	if err := writeGeneration(owner.root, filepath.Join(owner.root, "state-"+name), raw); err != nil {
		return err
	}
	if _, err := readClosedSetGeneration(owner.root, name); err != nil {
		return err
	}
	if err := replaceWatermark(owner.root, next.Generation, name); err != nil {
		return err
	}
	if err := replaceCurrent(owner.root, name); err != nil {
		return err
	}
	if err := cleanupGenerations(owner.root, name, next.Previous); err != nil {
		return err
	}
	owner.state, owner.name = next, name
	return nil
}

func cleanupClosedSetStaging(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".stage-") || strings.HasPrefix(name, ".current-") || strings.HasPrefix(name, ".watermark-") {
			if err := os.Remove(filepath.Join(root, name)); err != nil {
				return err
			}
		}
	}
	return syncDirectory(root)
}
