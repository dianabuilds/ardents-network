package releasedecision

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
)

// encodeFloorGeneration renders the supplied floor set as a stable byte
// payload.
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
			return nil, fmt.Errorf("releasedecision: unknown role %q", role)
		}
		if role != "root" && version == 0 && len(digest) == 0 {
			continue
		}
		if version <= 0 || len(digest) != 32 {
			return nil, fmt.Errorf("releasedecision: role %q floor is incomplete", role)
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

// readFloorGeneration reads the named generation under the state root
// and returns its floor set.
func readFloorGeneration(root, name string) (FloorSet, error) {
	if !floorGenerationName.MatchString(name) {
		return FloorSet{}, errors.New("releasedecision: generation name is invalid")
	}
	return readFloorGenerationFromPath(filepath.Join(root, "generations", name))
}

// readFloorGenerationFromPath decodes the state.bin of an existing
// generation directory into a floor set.
func readFloorGenerationFromPath(directory string) (FloorSet, error) {
	entries, err := readFloorStoreDirectory(directory, 2)
	if err != nil {
		return FloorSet{}, err
	}
	if len(entries) != 2 {
		return FloorSet{}, errors.New("releasedecision: generation inventory is incomplete")
	}
	seen := map[string]bool{}
	for _, entry := range entries {
		seen[entry.Name()] = true
	}
	if !seen["state.bin"] || !seen["roots"] {
		return FloorSet{}, errors.New("releasedecision: generation inventory contains an unknown entry")
	}
	payload, err := readBoundedFloorFile(filepath.Join(directory, "state.bin"), floorFileSizeLimit)
	if err != nil {
		return FloorSet{}, fmt.Errorf("releasedecision: read generation state: %w", err)
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
		return FloorSet{}, errors.New("releasedecision: generation state is not canonical")
	}
	if err := validateStoredRootArchive(directory, floors); err != nil {
		return FloorSet{}, err
	}
	return floors, nil
}

// decodeFloorGeneration parses the canonical state.bin encoding.
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
			return FloorSet{}, errors.New("releasedecision: floor role is unknown or duplicated")
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

// parseFloorLine parses one line of the canonical state.bin encoding.
func parseFloorLine(line []byte) (role string, version int64, digest []byte, err error) {
	if !bytes.HasPrefix(line, []byte(floorDigestPrefix)) {
		return "", 0, nil, errors.New("releasedecision: floor line is missing the role prefix")
	}
	rest := line[len(floorDigestPrefix):]
	roleEnd := bytes.IndexByte(rest, ' ')
	if roleEnd < 0 {
		return "", 0, nil, errors.New("releasedecision: floor line is missing the version field")
	}
	role = string(rest[:roleEnd])
	rest = rest[roleEnd+1:]
	if !bytes.HasPrefix(rest, []byte("version=")) {
		return "", 0, nil, errors.New("releasedecision: floor line is missing the version field")
	}
	rest = rest[len("version="):]
	versionEnd := bytes.IndexByte(rest, ' ')
	if versionEnd < 0 {
		return "", 0, nil, errors.New("releasedecision: floor line is missing the digest field")
	}
	version, err = strconv.ParseInt(string(rest[:versionEnd]), 10, 64)
	if err != nil {
		return "", 0, nil, fmt.Errorf("releasedecision: floor line has an invalid version: %w", err)
	}
	rest = rest[versionEnd+1:]
	if !bytes.HasPrefix(rest, []byte("digest=")) {
		return "", 0, nil, errors.New("releasedecision: floor line is missing the digest field")
	}
	encoded := rest[len("digest="):]
	if len(encoded) != 64 {
		return "", 0, nil, errors.New("releasedecision: floor digest has the wrong length")
	}
	digest, err = hex.DecodeString(string(encoded))
	if err != nil {
		return "", 0, nil, fmt.Errorf("releasedecision: floor digest is invalid: %w", err)
	}
	return role, version, digest, nil
}

// floorSetEqual reports whether two floor sets are byte-for-byte equal.
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
