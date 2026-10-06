package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func encodeFloorGeneration(floors FloorSet) ([]byte, error) {
	var buffer bytes.Buffer
	for _, role := range floorRoles {
		var version int64
		var digest []byte
		switch role {
		case "root":
			version, digest = floors.RootVersion, floors.RootDigest
		case "timestamp":
			version, digest = floors.TimestampVersion, floors.TimestampDigest
		case "snapshot":
			version, digest = floors.SnapshotVersion, floors.SnapshotDigest
		case "targets":
			version, digest = floors.TargetsVersion, floors.TargetsDigest
		default:
			return nil, fmt.Errorf("release: unknown role %q", role)
		}
		if role != "root" && version == 0 && len(digest) == 0 {
			continue
		}
		if version <= 0 || len(digest) != 32 {
			return nil, fmt.Errorf("release: role %q floor is incomplete", role)
		}
		buffer.WriteString(floorDigestPrefix)
		buffer.WriteString(role)
		buffer.WriteString(" version=")
		buffer.WriteString(strconv.FormatInt(version, 10))
		buffer.WriteString(" digest=")
		buffer.WriteString(hex.EncodeToString(digest))
		buffer.WriteByte('\n')
	}
	return buffer.Bytes(), nil
}

func readFloorGeneration(root, name string) (FloorSet, error) {
	if !floorGenerationName.MatchString(name) {
		return FloorSet{}, errors.New("release: generation name is invalid")
	}
	return readFloorGenerationFromPath(filepath.Join(root, "generations", name))
}

func readFloorGenerationFromPath(directory string) (FloorSet, error) {
	entries, err := readFloorStoreDirectory(directory, 2)
	if err != nil {
		return FloorSet{}, err
	}
	if len(entries) != 2 {
		return FloorSet{}, errors.New("release: generation inventory is incomplete")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Name()] = true
	}
	if !seen["state.bin"] || !seen["roots"] {
		return FloorSet{}, errors.New("release: generation inventory contains an unknown entry")
	}
	payload, err := readBoundedFloorFile(filepath.Join(directory, "state.bin"), floorFileSizeLimit)
	if err != nil {
		return FloorSet{}, fmt.Errorf("release: read generation state: %w", err)
	}
	floors, err := decodeFloorGeneration(payload)
	if err != nil {
		return FloorSet{}, err
	}
	if err := validateFloorSet(floors); err != nil {
		return FloorSet{}, err
	}
	canonical, err := encodeFloorGeneration(floors)
	if err != nil || !bytes.Equal(payload, canonical) {
		return FloorSet{}, errors.New("release: generation state is not canonical")
	}
	roots, err := validateStoredRootArchive(directory, floors)
	if err != nil {
		return FloorSet{}, err
	}
	if filepath.Base(directory) != floorGenerationID(payload, roots) {
		return FloorSet{}, errors.New("release: generation identity differs from retained bytes")
	}
	return floors, nil
}

func decodeFloorGeneration(payload []byte) (FloorSet, error) {
	parsed := make(map[string]int64)
	digests := make(map[string][]byte)
	known := map[string]bool{"root": true, "timestamp": true, "snapshot": true, "targets": true}
	for _, line := range bytes.Split(bytes.TrimRight(payload, "\n"), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		role, version, digest, err := parseFloorLine(line)
		if err != nil {
			return FloorSet{}, err
		}
		if !known[role] || parsed[role] != 0 {
			return FloorSet{}, errors.New("release: floor role is unknown or duplicated")
		}
		parsed[role] = version
		digests[role] = digest
	}
	var floors FloorSet
	for _, role := range floorRoles {
		switch role {
		case "root":
			floors.RootVersion = parsed[role]
			floors.RootDigest = digests[role]
		case "timestamp":
			floors.TimestampVersion = parsed[role]
			floors.TimestampDigest = digests[role]
		case "snapshot":
			floors.SnapshotVersion = parsed[role]
			floors.SnapshotDigest = digests[role]
		case "targets":
			floors.TargetsVersion = parsed[role]
			floors.TargetsDigest = digests[role]
		}
	}
	return floors, nil
}

func parseFloorLine(line []byte) (role string, version int64, digest []byte, err error) {
	if !bytes.HasPrefix(line, []byte(floorDigestPrefix)) {
		return "", 0, nil, errors.New("release: floor line is missing the role prefix")
	}
	rest := line[len(floorDigestPrefix):]
	roleEnd := bytes.IndexByte(rest, ' ')
	if roleEnd < 0 {
		return "", 0, nil, errors.New("release: floor line is missing the version field")
	}
	role = string(rest[:roleEnd])
	rest = rest[roleEnd+1:]
	if !bytes.HasPrefix(rest, []byte("version=")) {
		return "", 0, nil, errors.New("release: floor line is missing the version field")
	}
	rest = rest[len("version="):]
	versionEnd := bytes.IndexByte(rest, ' ')
	if versionEnd < 0 {
		return "", 0, nil, errors.New("release: floor line is missing the digest field")
	}
	version, err = strconv.ParseInt(string(rest[:versionEnd]), 10, 64)
	if err != nil {
		return "", 0, nil, fmt.Errorf("release: floor line has an invalid version: %w", err)
	}
	rest = rest[versionEnd+1:]
	if !bytes.HasPrefix(rest, []byte("digest=")) {
		return "", 0, nil, errors.New("release: floor line is missing the digest field")
	}
	encoded := rest[len("digest="):]
	if len(encoded) != 64 {
		return "", 0, nil, errors.New("release: floor digest has the wrong length")
	}
	digest, err = hex.DecodeString(string(encoded))
	if err != nil {
		return "", 0, nil, fmt.Errorf("release: floor digest is invalid: %w", err)
	}
	return role, version, digest, nil
}

func floorSetEqual(a, b FloorSet) bool {
	return a.RootVersion == b.RootVersion &&
		bytes.Equal(a.RootDigest, b.RootDigest) &&
		a.TimestampVersion == b.TimestampVersion &&
		bytes.Equal(a.TimestampDigest, b.TimestampDigest) &&
		a.SnapshotVersion == b.SnapshotVersion &&
		bytes.Equal(a.SnapshotDigest, b.SnapshotDigest) &&
		a.TargetsVersion == b.TargetsVersion &&
		bytes.Equal(a.TargetsDigest, b.TargetsDigest)
}

type rootArchiveEntry struct {
	version int64
	bytes   []byte
}

func validateRootArchive(chain [][]byte, floors FloorSet) ([]rootArchiveEntry, error) {
	if len(chain) == 0 || len(chain) > int(maximumRootRotations)+1 {
		return nil, errors.New("release: root archive has an invalid length")
	}
	entries := make([]rootArchiveEntry, 0, len(chain))
	var previous int64
	for _, data := range chain {
		if len(data) == 0 || int64(len(data)) > maximumMetadataFileBytes {
			return nil, errors.New("release: archived root exceeds the byte bound")
		}
		root, err := metadata.Root().FromBytes(data)
		if err != nil {
			return nil, fmt.Errorf("release: decode archived root: %w", err)
		}
		version := root.Signed.Version
		if previous != 0 && version != previous+1 {
			return nil, errors.New("release: archived roots are not consecutive")
		}
		entries = append(entries, rootArchiveEntry{version: version, bytes: append([]byte(nil), data...)})
		previous = version
	}
	last := entries[len(entries)-1]
	digest := sha256.Sum256(last.bytes)
	if last.version != floors.RootVersion || !bytes.Equal(digest[:], floors.RootDigest) {
		return nil, errors.New("release: archived final root disagrees with the floor")
	}
	return entries, nil
}

func writeRootArchive(staging string, entries []rootArchiveEntry) error {
	directory := filepath.Join(staging, "roots")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return fmt.Errorf("release: create root archive: %w", err)
	}
	for _, entry := range entries {
		name := strconv.FormatInt(entry.version, 10) + ".root.json"
		if err := writeSyncedFile(filepath.Join(directory, name), entry.bytes); err != nil {
			return err
		}
	}
	return syncDirectory(directory)
}

func validateStoredRootArchive(directory string, floors FloorSet) ([]rootArchiveEntry, error) {
	rootDirectory := filepath.Join(directory, "roots")
	entries, err := readFloorStoreDirectory(rootDirectory, int(maximumRootRotations)+1)
	if err != nil {
		return nil, fmt.Errorf("release: read root archive: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool {
		return rootVersionFromName(entries[i].Name()) < rootVersionFromName(entries[j].Name())
	})
	chain := make([][]byte, 0, len(entries))
	for _, entry := range entries {
		if !entry.Type().IsRegular() || !strings.HasSuffix(entry.Name(), ".root.json") {
			return nil, errors.New("release: root archive contains an invalid entry")
		}
		nameVersion := rootVersionFromName(entry.Name())
		if nameVersion == 1<<63-1 {
			return nil, errors.New("release: root archive filename is invalid")
		}
		data, err := readBoundedFloorFile(filepath.Join(rootDirectory, entry.Name()), maximumMetadataFileBytes)
		if err != nil {
			return nil, err
		}
		root, err := metadata.Root().FromBytes(data)
		if err != nil || root.Signed.Version != nameVersion {
			return nil, errors.New("release: root archive filename disagrees with its bytes")
		}
		chain = append(chain, data)
	}
	return validateRootArchive(chain, floors)
}

func rootVersionFromName(name string) int64 {
	versionText := strings.TrimSuffix(name, ".root.json")
	version, err := strconv.ParseInt(versionText, 10, 64)
	if err != nil || version <= 0 {
		return 1<<63 - 1
	}
	return version
}
