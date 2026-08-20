package releasedecision

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/theupdateframework/go-tuf/v2/metadata"
)

type rootArchiveEntry struct {
	version int64
	bytes   []byte
}

func validateRootArchive(chain [][]byte, floors FloorSet) ([]rootArchiveEntry, error) {
	if len(chain) == 0 || len(chain) > int(maximumRootRotations)+1 {
		return nil, errors.New("releasedecision: root archive has an invalid length")
	}
	entries := make([]rootArchiveEntry, 0, len(chain))
	var previous int64
	for _, data := range chain {
		if len(data) == 0 || int64(len(data)) > maximumMetadataFileBytes {
			return nil, errors.New("releasedecision: archived root exceeds the byte bound")
		}
		root, err := metadata.Root().FromBytes(data)
		if err != nil {
			return nil, fmt.Errorf("releasedecision: decode archived root: %w", err)
		}
		version := root.Signed.Version
		if previous != 0 && version != previous+1 {
			return nil, errors.New("releasedecision: archived roots are not consecutive")
		}
		entries = append(entries, rootArchiveEntry{version: version, bytes: append([]byte(nil), data...)})
		previous = version
	}
	last := entries[len(entries)-1]
	digest := sha256.Sum256(last.bytes)
	if last.version != floors.RootVersion || !bytes.Equal(digest[:], floors.RootDigest) {
		return nil, errors.New("releasedecision: archived final root disagrees with the floor")
	}
	return entries, nil
}

func writeRootArchive(staging string, entries []rootArchiveEntry) error {
	directory := filepath.Join(staging, "roots")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return fmt.Errorf("releasedecision: create root archive: %w", err)
	}
	for _, entry := range entries {
		name := strconv.FormatInt(entry.version, 10) + ".root.json"
		if err := writeSyncedFile(filepath.Join(directory, name), entry.bytes); err != nil {
			return err
		}
	}
	return syncDirectory(directory)
}

func validateStoredRootArchive(directory string, floors FloorSet) error {
	rootDirectory := filepath.Join(directory, "roots")
	entries, err := readFloorStoreDirectory(rootDirectory, int(maximumRootRotations)+1)
	if err != nil {
		return fmt.Errorf("releasedecision: read root archive: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return rootVersionFromName(entries[i].Name()) < rootVersionFromName(entries[j].Name())
	})
	chain := make([][]byte, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".root.json") {
			return errors.New("releasedecision: root archive contains an invalid entry")
		}
		nameVersion := rootVersionFromName(entry.Name())
		if nameVersion == 1<<63-1 {
			return errors.New("releasedecision: root archive filename is invalid")
		}
		data, err := readBoundedFloorFile(filepath.Join(rootDirectory, entry.Name()), maximumMetadataFileBytes)
		if err != nil {
			return err
		}
		root, err := metadata.Root().FromBytes(data)
		if err != nil || root.Signed.Version != nameVersion {
			return errors.New("releasedecision: root archive filename disagrees with its bytes")
		}
		chain = append(chain, data)
	}
	_, err = validateRootArchive(chain, floors)
	return err
}

func rootVersionFromName(name string) int64 {
	versionText := strings.TrimSuffix(name, ".root.json")
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || version <= 0 {
		return 1<<63 - 1
	}
	return version
}
