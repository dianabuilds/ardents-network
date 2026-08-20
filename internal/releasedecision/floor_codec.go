package releasedecision

import (
	"bytes"
	"errors"
	"fmt"
	"os"
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
		if version <= 0 || len(digest) != 32 {
			return nil, fmt.Errorf("releasedecision: role %q floor is incomplete", role)
		}
		buffer.WriteString(floorDigestPrefix)
		buffer.WriteString(role)
		buffer.WriteString(" version=")
		buffer.WriteString(strconv.FormatInt(version, 10))
		buffer.WriteString(" digest=")
		buffer.WriteString(hexEncode(digest))
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
	payload, err := readBoundedFloorFile(filepath.Join(directory, "state.bin"), floorFileSizeLimit)
	if err != nil {
		return FloorSet{}, fmt.Errorf("releasedecision: read generation state: %w", err)
	}
	return decodeFloorGeneration(payload)
}

// decodeFloorGeneration parses the canonical state.bin encoding.
func decodeFloorGeneration(payload []byte) (FloorSet, error) {
	parsed := make(map[string]int64)
	digests := make(map[string][]byte)
	for _, line := range bytes.Split(bytes.TrimRight(payload, "\n"), []byte{'\n'}) {
		if len(line) == 0 {
			continue
		}
		role, version, digest, err := parseFloorLine(line)
		if err != nil {
			return FloorSet{}, err
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
	digest, err = hexDecode(encoded)
	if err != nil {
		return "", 0, nil, fmt.Errorf("releasedecision: floor digest is invalid: %w", err)
	}
	return role, version, digest, nil
}

// publishFloorGeneration stages the payload under a hidden directory,
// then renames it atomically into the generations tree.
func publishFloorGeneration(generations, name string, payload []byte) error {
	staging, err := os.MkdirTemp(generations, ".stage-")
	if err != nil {
		return fmt.Errorf("releasedecision: create generation staging: %w", err)
	}
	if err := writeSyncedFile(filepath.Join(staging, "state.bin"), payload); err != nil {
		_ = os.RemoveAll(staging)
		return err
	}
	final := filepath.Join(generations, name)
	if err := os.Rename(staging, final); err != nil {
		_ = os.RemoveAll(staging)
		return fmt.Errorf("releasedecision: publish generation: %w", err)
	}
	// The atomic rename is the durable part; the directory sync is
	// best-effort. Windows may report ACCESS_DENIED while the renamed
	// directory entry is being closed by the kernel.
	_ = syncDirectory(generations)
	return nil
}

// writeFloorStorePointer writes the supplied generation name as the
// current pointer using an atomic file replace.
func writeFloorStorePointer(root, name string) error {
	temporary, err := os.CreateTemp(root, ".current-")
	if err != nil {
		return fmt.Errorf("releasedecision: create current pointer staging: %w", err)
	}
	path := temporary.Name()
	defer func() { _ = os.Remove(path) }()
	if err = temporary.Chmod(0o600); err == nil {
		_, err = temporary.WriteString(name + "\n")
	}
	if err == nil {
		err = temporary.Sync()
	}
	closeErr := temporary.Close()
	if err != nil {
		return fmt.Errorf("releasedecision: write current pointer: %w", err)
	}
	if closeErr != nil {
		return fmt.Errorf("releasedecision: close current pointer: %w", closeErr)
	}
	if err := os.Rename(path, filepath.Join(root, "current")); err != nil {
		return fmt.Errorf("releasedecision: replace current pointer: %w", err)
	}
	_ = syncDirectory(root)
	return nil
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

// floorGenerationID returns a stable eight-hex-character id derived
// from the SHA-256 of the generation payload.
func floorGenerationID(payload []byte) string {
	digest := sha256Sum(payload)
	return hexEncode(digest[:4])
}
