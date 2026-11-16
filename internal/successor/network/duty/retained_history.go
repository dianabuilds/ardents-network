package duty

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func inspectRetainedEntries(root string) (result error) {
	directory, err := os.Open(root)
	if err != nil {
		return err
	}
	defer func() { result = errors.Join(result, directory.Close()) }()
	entries, err := directory.ReadDir(10)
	if err != nil && err != io.EOF {
		return err
	}
	if len(entries) > 9 {
		return errors.New("retained participation inventory exceeds bound")
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") && entry.Name() != rootLockName && entry.Name() != rootMarkerName {
			return errors.New("retained participation has an uncertain prefix")
		}
		info, err := os.Lstat(filepath.Join(root, entry.Name()))
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
			return errors.New("retained participation record access differs")
		}
	}
	return nil
}

func verifyRetainedHistory(root string) error {
	if err := inspectRetainedEntries(root); err != nil {
		return err
	}
	generation, name, present, err := loadWatermark(root)
	if err != nil || !present {
		return errors.New("retained participation watermark is absent or invalid")
	}
	pointer, err := readBounded(filepath.Join(root, "current"), 65)
	if err != nil || string(pointer) != name+"\n" {
		return errors.New("retained participation pointer requires recovery")
	}
	state, err := loadGeneration(root, name)
	if err != nil || state.Generation != generation {
		return errors.New("retained participation generation differs")
	}
	return nil
}

// CheckSourceProducer verifies the exact retained serving obligation without a
// Claim probe or write. Any other record refuses this serving-only repair.
// Native process death remains the State
// repair adapter's independent prerequisite; this method grants no repair right.
func (store *store) CheckSourceProducer(producer [32]byte, expected Duty) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.closed || store.failed != nil || producer == ([32]byte{}) || expected.Class != "direct-source" || expected.State != "live" {
		return errors.New("Source producer inspection is unavailable")
	}
	found := false
	for _, record := range store.state.Duties {
		if record.Producer == producer {
			if found || record.Identity != expected.Identity || record.Family != expected.Family || record.Class != expected.Class || record.State != expected.State || record.NotAfter != expected.NotAfter.Unix() {
				return errors.New("retained Source serving obligation differs")
			}
			found = true
		} else {
			return errors.New("another unresolved participation obligation remains")
		}
	}
	if !found {
		return errors.New("no retained Source serving producer")
	}
	return nil
}
