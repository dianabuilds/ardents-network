//go:build live

package network_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maximumFinalHandoffArtifact = 16 << 20

type finalWorkerObserverEvidence struct {
	Schema   string                `json:"schema"`
	CellID   string                `json:"cell_id"`
	Sets     []finalRawObserverSet `json:"sets"`
	Exercise *finalFaultExercise   `json:"exercise,omitempty"`
}

type finalWorkerTelemetryEvidence struct {
	Schema string                     `json:"schema"`
	CellID string                     `json:"cell_id"`
	Files  []finalWorkerTelemetryFile `json:"files"`
}

type finalWorkerTelemetryFile struct {
	Root     uint16              `json:"root"`
	Role     string              `json:"role"`
	Kind     string              `json:"kind"`
	Artifact finalRunnerArtifact `json:"artifact"`
}

func writeFinalWorkerHandoff(root, cell string, observers []finalRawObserverSet,
	telemetry []finalRawTelemetry,
) error {
	if !filepath.IsAbs(root) || cell == "" {
		return errors.New("final worker handoff root is invalid")
	}
	directory := filepath.Join(root, "handoff")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return err
	}
	observer := finalWorkerObserverEvidence{Schema: "ardents-h3-final-raw-observers-v1",
		CellID: cell, Sets: observers, Exercise: consumeFinalFaultExercise(cell)}
	if strings.HasPrefix(cell, "hostile/") && observer.Exercise == nil {
		return errors.New("hostile final worker omitted its external fault exercise")
	}
	if err := writeFinalWorkerArtifact(filepath.Join(directory, "observers.json"), observer); err != nil {
		return err
	}
	if !validFinalWorkerTelemetryInventory(telemetry, cell) {
		return errors.New("final worker telemetry inventory differs from the cell contract")
	}
	telemetryDirectory := filepath.Join(directory, "telemetry")
	if err := os.Mkdir(telemetryDirectory, 0o700); err != nil {
		return err
	}
	files := make([]finalWorkerTelemetryFile, 0, len(telemetry))
	for index, stream := range telemetry {
		if len(stream.Data) == 0 || len(stream.Data) > maximumFinalTelemetryFile {
			return errors.New("final worker telemetry stream is empty or oversized")
		}
		relative := finalWorkerTelemetryPath(index)
		artifact, err := writeFinalWorkerBytes(filepath.Join(directory, filepath.FromSlash(relative)),
			stream.Data, maximumFinalTelemetryFile)
		if err != nil {
			return err
		}
		artifact.Path = relative
		files = append(files, finalWorkerTelemetryFile{Root: stream.Root, Role: stream.Role,
			Kind: stream.Kind, Artifact: artifact})
	}
	telemetryValue := finalWorkerTelemetryEvidence{Schema: "ardents-h3-final-raw-telemetry-v2",
		CellID: cell, Files: files}
	return writeFinalWorkerArtifact(filepath.Join(directory, "telemetry.json"), telemetryValue)
}

func validFinalWorkerTelemetryInventory(files []finalRawTelemetry, cell string) bool {
	wanted := finalWorkerTelemetryLayout(cell)
	if len(files) != len(wanted) {
		return false
	}
	for index, expected := range wanted {
		file := files[index]
		if file.Root != expected.root || file.Role != expected.role || file.Kind != expected.kind {
			return false
		}
	}
	return true
}

type finalTelemetrySlot struct {
	root uint16
	role string
	kind string
}

func finalWorkerTelemetryLayout(cell string) []finalTelemetrySlot {
	var result []finalTelemetrySlot
	appendRole := func(root uint16, role string, tree bool) {
		kinds := []string{"resource.jsonl", "carrier.jsonl"}
		if tree {
			kinds = append(kinds, "tree.jsonl")
			if role == "bridge" {
				kinds = append(kinds, "runtime.jsonl")
			}
		}
		for _, kind := range kinds {
			result = append(result, finalTelemetrySlot{root: root, role: role, kind: kind})
		}
	}
	capacity := 0
	if strings.HasPrefix(cell, "capacity/h3-s5-b1-v1-strong/") {
		capacity = 16
	} else if strings.HasPrefix(cell, "capacity/h3-s5-b1-v1/") {
		capacity = 4
	}
	if capacity > 0 {
		for root := range capacity {
			appendRole(uint16(root), "endpoint", true)
		}
		appendRole(0, "bridge", true)
		appendRole(0, "publisher", true)
		return result
	}
	if strings.HasPrefix(cell, "pressure/") {
		roots := 1
		if cell == "pressure/P4" {
			roots = 10
		}
		for root := range roots {
			result = append(result, finalTelemetrySlot{root: uint16(root), role: "bridge", kind: "resource.jsonl"})
			if cell == "pressure/P0" || cell == "pressure/P1" || cell == "pressure/P4" {
				result = append(result, finalTelemetrySlot{root: uint16(root), role: "bridge", kind: "pressure-input.json"})
			} else {
				result = append(result, finalTelemetrySlot{root: uint16(root), role: "pressure", kind: "pressure-injection.jsonl"})
				result = append(result, finalTelemetrySlot{root: uint16(root), role: "bridge", kind: "pressure-state.jsonl"})
			}
		}
		return append(result, finalTelemetrySlot{root: 0, role: "bridge", kind: "pressure.json"})
	}
	roots := 1
	for root := range roots {
		for _, role := range []string{"endpoint", "bridge", "publisher"} {
			appendRole(uint16(root), role, strings.HasPrefix(cell, "sustained/"))
		}
	}
	if strings.HasPrefix(cell, "recovery/") {
		result = append(result, finalTelemetrySlot{root: 0, role: "bridge", kind: "recovery.json"})
	}
	return result
}

func publishFinalWorkerHandoff(root, secret, cell string) (finalRunnerArtifact, finalRunnerArtifact, error) {
	if !validFinalWorkerHandoffRoot(root, secret) || cell == "" {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, errors.New("final worker handoff ownership is invalid")
	}
	rootAliased, rootAliasErr := finalPathHasSymlink(root)
	secretAliased, secretAliasErr := finalPathHasSymlink(secret)
	if rootAliasErr != nil || secretAliasErr != nil || rootAliased || secretAliased {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, errors.Join(rootAliasErr, secretAliasErr,
			errors.New("final worker handoff path is aliased"))
	}
	handoff := filepath.Join(root, "handoff")
	var observers finalWorkerObserverEvidence
	observerRaw, err := readFinalWorkerArtifact(filepath.Join(handoff, "observers.json"), &observers)
	if err != nil || observers.Schema != "ardents-h3-final-raw-observers-v1" || observers.CellID != cell {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, errors.Join(err,
			errors.New("final worker observer handoff is invalid"))
	}
	var telemetry finalWorkerTelemetryEvidence
	telemetryRaw, err := readFinalWorkerArtifact(filepath.Join(handoff, "telemetry.json"), &telemetry)
	if err != nil || telemetry.Schema != "ardents-h3-final-raw-telemetry-v2" || telemetry.CellID != cell ||
		!validFinalWorkerTelemetryIndex(telemetry.Files, cell) {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, errors.Join(err,
			errors.New("final worker telemetry handoff is invalid"))
	}
	published := telemetry
	for index, stream := range telemetry.Files {
		if stream.Artifact.Path != finalWorkerTelemetryPath(index) || stream.Artifact.Bytes < 1 ||
			stream.Artifact.Bytes > maximumFinalTelemetryFile {
			return finalRunnerArtifact{}, finalRunnerArtifact{}, errors.New("final worker telemetry commitment is invalid")
		}
		raw, readErr := readFinalWorkerBytes(filepath.Join(handoff,
			filepath.FromSlash(stream.Artifact.Path)), stream.Artifact, maximumFinalTelemetryFile)
		if readErr != nil {
			return finalRunnerArtifact{}, finalRunnerArtifact{}, readErr
		}
		artifact, publishErr := publishFinalArtifactAt(secret, finalTelemetryStreamPath(cell, index), raw)
		if publishErr != nil {
			return finalRunnerArtifact{}, finalRunnerArtifact{}, publishErr
		}
		published.Files[index].Artifact = artifact
	}
	telemetryRaw, err = canonicalFinalWorkerArtifact(published)
	if err != nil {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, err
	}
	observerArtifact, err := publishFinalArtifact(secret, "final-observers", cell, observerRaw)
	if err != nil {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, err
	}
	telemetryArtifact, err := publishFinalArtifact(secret, "final-telemetry", cell, telemetryRaw)
	if err != nil {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, err
	}
	for index := range telemetry.Files {
		if err := os.Remove(filepath.Join(handoff, filepath.FromSlash(finalWorkerTelemetryPath(index)))); err != nil {
			return finalRunnerArtifact{}, finalRunnerArtifact{}, err
		}
	}
	if err := errors.Join(os.Remove(filepath.Join(handoff, "observers.json")),
		os.Remove(filepath.Join(handoff, "telemetry.json")), os.Remove(filepath.Join(handoff, "telemetry")),
		os.Remove(handoff)); err != nil {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, err
	}
	return observerArtifact, telemetryArtifact, nil
}

func validFinalWorkerTelemetryIndex(files []finalWorkerTelemetryFile, cell string) bool {
	values := make([]finalRawTelemetry, len(files))
	for index, file := range files {
		values[index] = finalRawTelemetry{Root: file.Root, Role: file.Role, Kind: file.Kind, Data: []byte{1}}
	}
	return validFinalWorkerTelemetryInventory(values, cell)
}

func writeFinalWorkerArtifact(path string, value any) error {
	raw, err := canonicalFinalWorkerArtifact(value)
	if err != nil {
		return err
	}
	_, err = writeFinalWorkerBytes(path, raw, maximumFinalHandoffArtifact)
	return err
}

func canonicalFinalWorkerArtifact(value any) ([]byte, error) {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw) == 0 || len(raw)+1 > maximumFinalHandoffArtifact {
		return nil, errors.Join(err, errors.New("final worker artifact is empty or oversized"))
	}
	return append(raw, '\n'), nil
}

func writeFinalWorkerBytes(path string, raw []byte, maximum int) (finalRunnerArtifact, error) {
	if len(raw) == 0 || len(raw) > maximum {
		return finalRunnerArtifact{}, errors.New("final worker bytes are empty or oversized")
	}
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return finalRunnerArtifact{}, err
	}
	written, writeErr := output.Write(raw)
	syncErr, closeErr := output.Sync(), output.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil || written != len(raw) {
		return finalRunnerArtifact{}, errors.Join(writeErr, syncErr, closeErr,
			errors.New("final worker artifact write is incomplete"))
	}
	digest := sha256.Sum256(raw)
	return finalRunnerArtifact{SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(raw))}, nil
}

func readFinalWorkerArtifact(path string, value any) ([]byte, error) {
	raw, err := readFinalWorkerBounded(path, maximumFinalHandoffArtifact)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("final worker artifact is not one strict value")
	}
	canonical, err := json.MarshalIndent(value, "", "  ")
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return nil, errors.New("final worker artifact is not canonical JSON")
	}
	return raw, nil
}

func readFinalWorkerBytes(path string, artifact finalRunnerArtifact, maximum int) ([]byte, error) {
	if artifact.Bytes < 1 || artifact.Bytes > int64(maximum) || len(artifact.SHA256) != 64 {
		return nil, errors.New("final worker byte commitment is invalid")
	}
	raw, err := readFinalWorkerBounded(path, maximum)
	if err != nil || int64(len(raw)) != artifact.Bytes {
		return nil, errors.Join(err, errors.New("final worker byte size differs from commitment"))
	}
	digest := sha256.Sum256(raw)
	if hex.EncodeToString(digest[:]) != artifact.SHA256 {
		return nil, errors.New("final worker byte hash differs from commitment")
	}
	return raw, nil
}

func readFinalWorkerBounded(path string, maximum int) ([]byte, error) {
	aliased, aliasErr := finalPathHasSymlink(path)
	if aliasErr != nil || aliased {
		return nil, errors.Join(aliasErr, errors.New("final worker artifact path is aliased"))
	}
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() ||
		before.Size() < 1 || before.Size() > int64(maximum) {
		return nil, errors.Join(err, errors.New("final worker artifact is not a bounded regular file"))
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(input, int64(maximum)+1))
	after, statErr := input.Stat()
	closeErr := input.Close()
	if readErr != nil || statErr != nil || closeErr != nil || len(raw) > maximum ||
		!os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(readErr, statErr, closeErr, errors.New("final worker artifact changed while reading"))
	}
	return raw, nil
}

func publishFinalArtifact(secret, directory, cell string, raw []byte) (finalRunnerArtifact, error) {
	relative := finalRunnerArtifactPath(directory, cell)
	return publishFinalArtifactAt(secret, relative, raw)
}

func publishFinalArtifactAt(secret, relative string, raw []byte) (finalRunnerArtifact, error) {
	target := filepath.Join(secret, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return finalRunnerArtifact{}, err
	}
	aliased, aliasErr := finalPathHasSymlink(filepath.Dir(target))
	if aliasErr != nil || aliased {
		return finalRunnerArtifact{}, errors.Join(aliasErr, errors.New("final artifact destination is aliased"))
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return finalRunnerArtifact{}, err
	}
	written, writeErr := output.Write(raw)
	syncErr, closeErr := output.Sync(), output.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil || written != len(raw) {
		_ = os.Remove(target)
		return finalRunnerArtifact{}, errors.Join(writeErr, syncErr, closeErr,
			errors.New("final artifact publication is incomplete"))
	}
	digest := sha256.Sum256(raw)
	return finalRunnerArtifact{Path: relative, SHA256: hex.EncodeToString(digest[:]), Bytes: int64(len(raw))}, nil
}

func finalWorkerTelemetryPath(index int) string {
	return filepath.ToSlash(filepath.Join("telemetry", fmt.Sprintf("%03d.jsonl", index)))
}

func finalTelemetryStreamPath(cell string, index int) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join("final-telemetry", hex.EncodeToString(digest[:]),
		fmt.Sprintf("%03d.jsonl", index)))
}

func finalRunnerArtifactPath(directory, cell string) string {
	digest := sha256.Sum256([]byte(cell))
	return filepath.ToSlash(filepath.Join(directory, hex.EncodeToString(digest[:])+".json"))
}

func validFinalArtifactCommitment(value finalRunnerArtifact, directory, cell string) bool {
	if value.Path != finalRunnerArtifactPath(directory, cell) || value.Bytes < 1 ||
		value.Bytes > maximumFinalHandoffArtifact || len(value.SHA256) != 64 {
		return false
	}
	_, err := hex.DecodeString(value.SHA256)
	return err == nil && value.SHA256 == strings.ToLower(value.SHA256)
}

func validFinalWorkerHandoffRoot(root, secret string) bool {
	staging := os.Getenv("ARDENTS_BLOCKED_STAGING_ROOT")
	if !filepath.IsAbs(root) || !filepath.IsAbs(secret) || !filepath.IsAbs(staging) {
		return false
	}
	parent := filepath.Clean(filepath.Join(staging, "workers"))
	clean := filepath.Clean(root)
	if filepath.Dir(clean) != parent || len(filepath.Base(clean)) != 24 {
		return false
	}
	_, err := hex.DecodeString(filepath.Base(clean))
	return err == nil
}

func finalPathHasSymlink(path string) (bool, error) {
	for current := filepath.Clean(path); ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return false, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return true, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			return false, nil
		}
	}
}
