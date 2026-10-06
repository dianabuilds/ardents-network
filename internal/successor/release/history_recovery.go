package release

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

var archiveRootName = regexp.MustCompile(`^[1-9][0-9]{0,18}\.root\.json$`)

// Recovery removes only bounded exact writer forms after committed history has
// been validated under the exclusive lease. Residue never authorizes a target
// or becomes a candidate current pointer. Every form is checked before removal.
func (s *floorStore) recoverWriterResidue() error {
	rootEntries, err := readFloorStoreDirectory(s.path, 70)
	if err != nil {
		return err
	}
	var pointers []string
	for _, entry := range rootEntries {
		if !stagedPointerName.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(s.path, entry.Name())
		raw, err := readBoundedFloorFile(path, 65)
		if err != nil {
			return err
		}
		if !pointerPrefix(raw) {
			return errors.New("release: malformed staged pointer")
		}
		pointers = append(pointers, path)
	}
	genRoot := filepath.Join(s.path, "generations")
	entries, err := readFloorStoreDirectory(genRoot, 64)
	if err != nil {
		return err
	}
	var stages []string
	for _, entry := range entries {
		if !stagedGenerationName.MatchString(entry.Name()) {
			continue
		}
		path := filepath.Join(genRoot, entry.Name())
		if err = inspectWriterStage(path); err != nil {
			return err
		}
		stages = append(stages, path)
	}
	// There are no unvalidated recursive paths in this list: every entry is a
	// direct child of this held root and its complete bounded tree was inspected.
	for _, path := range stages {
		if err = os.RemoveAll(path); err != nil {
			return err
		}
	}
	for _, path := range pointers {
		if err = os.Remove(path); err != nil {
			return err
		}
	}
	if len(stages) > 0 {
		if err = syncDirectory(genRoot); err != nil {
			return err
		}
	}
	if len(pointers) > 0 {
		return syncDirectory(s.path)
	}
	return nil
}
func pointerPrefix(raw []byte) bool {
	if len(raw) > 65 {
		return false
	}
	for i, b := range raw {
		if i == 64 {
			return b == '\n'
		}
		if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f') {
			return false
		}
	}
	return true
}
func inspectWriterStage(path string) error {
	entries, err := readFloorStoreDirectory(path, 2)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		switch entry.Name() {
		case "state.bin":
			raw, err := readBoundedFloorFile(filepath.Join(path, "state.bin"), floorFileSizeLimit)
			if err != nil {
				return err
			}
			if !floorPayloadPrefix(raw) {
				return errors.New("release: invalid staged state prefix")
			}
		case "roots":
			roots, err := readFloorStoreDirectory(filepath.Join(path, "roots"), int(maximumRootRotations)+1)
			if err != nil {
				return err
			}
			for _, root := range roots {
				if !archiveRootName.MatchString(root.Name()) {
					return errors.New("release: foreign staged root name")
				}
				if _, err = readBoundedFloorFile(filepath.Join(path, "roots", root.Name()), maximumMetadataFileBytes); err != nil {
					return err
				}
			}
		default:
			return errors.New("release: foreign staged generation entry")
		}
	}
	return nil
}

func floorPayloadPrefix(raw []byte) bool {
	for _, role := range floorRoles {
		literal := []byte("role=" + role + " version=")
		if len(raw) < len(literal) {
			return bytes.HasPrefix(literal, raw)
		}
		if !bytes.HasPrefix(raw, literal) {
			return false
		}
		raw = raw[len(literal):]
		if len(raw) == 0 {
			return true
		}
		i := 0
		for i < len(raw) && raw[i] >= '0' && raw[i] <= '9' {
			i++
		}
		if i == 0 || raw[0] == '0' || i > 19 {
			return false
		}
		if _, err := strconv.ParseInt(string(raw[:i]), 10, 64); err != nil {
			return false
		}
		raw = raw[i:]
		if len(raw) == 0 {
			return true
		}
		literal = []byte(" digest=")
		if len(raw) < len(literal) {
			return bytes.HasPrefix(literal, raw)
		}
		if !bytes.HasPrefix(raw, literal) {
			return false
		}
		raw = raw[len(literal):]
		for i := 0; i < 64; i++ {
			if len(raw) == 0 {
				return true
			}
			b := raw[0]
			if !(b >= '0' && b <= '9' || b >= 'a' && b <= 'f') {
				return false
			}
			raw = raw[1:]
		}
		if len(raw) == 0 {
			return true
		}
		if raw[0] != '\n' {
			return false
		}
		raw = raw[1:]
		if len(raw) == 0 {
			return true
		}
	}
	return false
}
