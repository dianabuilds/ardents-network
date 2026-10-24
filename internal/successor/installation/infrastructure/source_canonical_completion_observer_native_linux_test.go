//go:build linux && amd64 && network_signing_native

package infrastructure

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// This is an independently retained native image at an actual production entry,
// not proof that the manager accepted Reload or that the journal writer closed.
// In particular, the recorded predecessor is not an observed pre-Commit image.
func (p *sourcePhysicalCut) canonicalCompletedImage(records []sourceRecord, root, unit string) ([]byte, os.FileInfo, error) {
	inventory, directory, err := p.canonicalSealedInventory(records, root, false)
	if err != nil {
		return nil, nil, err
	}
	initial := len(p.canonical.Arguments) >= 2 && p.canonical.Arguments[1] == "install-initial-stopped"
	one := func(phase, path string) (sourceRecord, error) {
		var found sourceRecord
		count := 0
		for _, record := range records {
			if record.Phase == phase && record.Path == path {
				found, count = record, count+1
			}
		}
		if count != 1 {
			return found, errors.New("one exact original completed image record required")
		}
		return found, nil
	}
	image := [][]byte{inventory}
	var bindingSHA, unitSHA string
	for _, fixed := range []struct{ target, name, intent, written, predecessor string }{
		{filepath.Join(root, "binding.json"), "binding.json", "selection-intent", "selection-written", "predecessor-binding"},
		{unit, "rendered-source.service", "unit-intent", "unit-written", "predecessor-unit"},
	} {
		candidate, err := one("file-ready", filepath.Join(root, "generations", p.canonical.Generation, fixed.name))
		if err != nil {
			return nil, nil, err
		}
		phase := fixed.written
		if initial {
			phase = "file-ready"
		}
		published, err := one(phase, fixed.target)
		if err != nil || published.SHA256 != candidate.SHA256 || published.Mode != 0644 || published.UID != 0 || published.GID != 0 {
			return nil, nil, errors.Join(errors.New("completed publication must match sealed candidate"), err)
		}
		if initial {
			if err := sourcePhysicalBirthTriple(records, published, "file"); err != nil {
				return nil, nil, err
			}
		} else {
			intent, err := one(fixed.intent, fixed.target)
			if err != nil {
				return nil, nil, err
			}
			expected := published
			expected.Sequence, expected.Phase = published.Sequence-1, fixed.intent
			previous, previousErr := one(fixed.predecessor, fixed.target)
			if intent != expected || previousErr != nil || previous.Sequence >= intent.Sequence || previous.Device != published.Device || previous.Inode != published.Inode || previous.Mode != 0644 || previous.UID != 0 || previous.GID != 0 || !sourceCutObserverSHA(previous.SHA256) {
				return nil, nil, errors.Join(errors.New("original completed publication pair/predecessor inode differs"), previousErr)
			}
		}
		body, info, err := readSourceRecoveryHeldPayload(fixed.target, published)
		if err != nil || sourcePhysicalSHA(body) != candidate.SHA256 {
			return nil, nil, errors.Join(errors.New("actual original completed publication bytes/inode differ"), err)
		}
		native := info.Sys().(*syscall.Stat_t)
		raw, err := json.Marshal(sourcePhysicalSealedFile{Record: published, Size: info.Size(), Changed: native.Ctim, Modified: native.Mtim})
		if err != nil {
			return nil, nil, err
		}
		image = append(image, raw)
		if fixed.name == "binding.json" {
			bindingSHA = candidate.SHA256
		} else {
			unitSHA = candidate.SHA256
		}
	}
	last := records[len(records)-1]
	expected := sourceRecord{Sequence: len(records) - 1, Phase: "reload-intent", Path: unit, SHA256: unitSHA, Mode: 0644}
	if p.window == "journal-complete-before-close" || p.window == "pending-before-unlink" {
		expected.Phase, expected.Path, expected.SHA256 = "replacement-verified", filepath.Join(root, "binding.json"), bindingSHA
		if initial {
			expected.Phase = "stopped-verified"
		}
		reload, err := one("reload-intent", unit)
		want := sourceRecord{Sequence: expected.Sequence - 1, Phase: "reload-intent", Path: unit, SHA256: unitSHA, Mode: 0644}
		if err != nil || reload != want {
			return nil, nil, errors.Join(errors.New("original reload record must precede complete record"), err)
		}
	}
	if last != expected {
		return nil, nil, errors.New("actual completed frontier record differs")
	}
	return bytes.Join(image, []byte{'\n'}), directory, nil
}
