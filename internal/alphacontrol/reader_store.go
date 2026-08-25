package alphacontrol

import (
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const floorSize = 4 + 1 + 8 + 32 + 3*(8+32)

func prepareReaderRoot(root string) error {
	info, err := os.Lstat(root)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		if err := os.MkdirAll(root, 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(root, readerMarkerName), []byte(readerMarker), 0o600); err != nil {
			return err
		}
		return nil
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("alpha control reader root is not an owned directory")
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return os.WriteFile(filepath.Join(root, readerMarkerName), []byte(readerMarker), 0o600)
	}
	marker, err := os.ReadFile(filepath.Join(root, readerMarkerName))
	if err != nil || string(marker) != readerMarker {
		return errors.New("alpha control reader root marker is invalid")
	}
	for _, entry := range entries {
		if entry.Name() != readerMarkerName && entry.Name() != readerFloorName && entry.Name() != readerLockName {
			return errors.New("alpha control reader root has an unknown entry")
		}
	}
	return nil
}

func readFloor(path string) (Floor, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return Floor{}, nil
	}
	if err != nil || len(raw) != floorSize || string(raw[:4]) != "ACF1" || raw[4] != 1 {
		return Floor{}, errors.New("alpha control reader floor is invalid")
	}
	offset := 5
	result := Floor{CatalogGeneration: binary.BigEndian.Uint64(raw[offset : offset+8])}
	offset += 8
	copy(result.CatalogDigest[:], raw[offset:offset+32])
	offset += 32
	for index := range result.Components {
		result.Components[index].Generation = binary.BigEndian.Uint64(raw[offset : offset+8])
		offset += 8
		copy(result.Components[index].Digest[:], raw[offset:offset+32])
		offset += 32
	}
	if result.CatalogGeneration == 0 && result.CatalogDigest != [32]byte{} {
		return Floor{}, errors.New("alpha control reader floor is inconsistent")
	}
	if result.CatalogGeneration == 0 {
		for _, component := range result.Components {
			if component.Generation != 0 || component.Digest != [32]byte{} {
				return Floor{}, errors.New("alpha control reader floor is inconsistent")
			}
		}
	}
	for _, component := range result.Components {
		if (component.Generation == 0) != (component.Digest == [32]byte{}) {
			return Floor{}, errors.New("alpha control reader floor is inconsistent")
		}
	}
	return result, nil
}

func writeFloor(path string, floor Floor) error {
	raw := make([]byte, 0, floorSize)
	raw = append(raw, 'A', 'C', 'F', '1', 1)
	raw = binary.BigEndian.AppendUint64(raw, floor.CatalogGeneration)
	raw = append(raw, floor.CatalogDigest[:]...)
	for _, component := range floor.Components {
		raw = binary.BigEndian.AppendUint64(raw, component.Generation)
		raw = append(raw, component.Digest[:]...)
	}
	if len(raw) != floorSize {
		return errors.New("alpha control reader floor encoding is invalid")
	}
	temporary := path + ".next"
	file, err := os.OpenFile(temporary, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err = file.Write(raw); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := durableReaderRename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return fmt.Errorf("publish alpha control reader floor: %w", err)
	}
	if err := syncReaderDirectory(filepath.Dir(path)); err != nil {
		return fmt.Errorf("sync alpha control reader floor directory: %w", err)
	}
	return nil
}
