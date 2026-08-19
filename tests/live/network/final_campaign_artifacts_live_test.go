//go:build live

package network_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maximumFinalHandoffArtifact = 16 << 20

type finalWorkerObserverEvidence struct {
	Schema string                `json:"schema"`
	CellID string                `json:"cell_id"`
	Sets   []finalRawObserverSet `json:"sets"`
}

type finalWorkerTelemetryEvidence struct {
	Schema string              `json:"schema"`
	CellID string              `json:"cell_id"`
	Files  []finalRawTelemetry `json:"files"`
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
		CellID: cell, Sets: observers}
	if err := writeFinalWorkerArtifact(filepath.Join(directory, "observers.json"), observer); err != nil {
		return err
	}
	telemetryValue := finalWorkerTelemetryEvidence{Schema: "ardents-h3-final-raw-telemetry-v1",
		CellID: cell, Files: telemetry}
	return writeFinalWorkerArtifact(filepath.Join(directory, "telemetry.json"), telemetryValue)
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
	if err != nil || telemetry.Schema != "ardents-h3-final-raw-telemetry-v1" || telemetry.CellID != cell {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, errors.Join(err,
			errors.New("final worker telemetry handoff is invalid"))
	}
	observerArtifact, err := publishFinalArtifact(secret, "final-observers", cell, observerRaw)
	if err != nil {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, err
	}
	telemetryArtifact, err := publishFinalArtifact(secret, "final-telemetry", cell, telemetryRaw)
	if err != nil {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, err
	}
	if err := errors.Join(os.Remove(filepath.Join(handoff, "observers.json")),
		os.Remove(filepath.Join(handoff, "telemetry.json")), os.Remove(handoff)); err != nil {
		return finalRunnerArtifact{}, finalRunnerArtifact{}, err
	}
	return observerArtifact, telemetryArtifact, nil
}

func writeFinalWorkerArtifact(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil || len(raw) == 0 || len(raw)+1 > maximumFinalHandoffArtifact {
		return errors.Join(err, errors.New("final worker artifact is empty or oversized"))
	}
	raw = append(raw, '\n')
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	written, writeErr := output.Write(raw)
	syncErr, closeErr := output.Sync(), output.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil || written != len(raw) {
		return errors.Join(writeErr, syncErr, closeErr, errors.New("final worker artifact write is incomplete"))
	}
	return nil
}

func readFinalWorkerArtifact(path string, value any) ([]byte, error) {
	aliased, aliasErr := finalPathHasSymlink(path)
	if aliasErr != nil || aliased {
		return nil, errors.Join(aliasErr, errors.New("final worker artifact path is aliased"))
	}
	before, err := os.Lstat(path)
	if err != nil || before.Mode()&os.ModeSymlink != 0 || !before.Mode().IsRegular() ||
		before.Size() < 1 || before.Size() > maximumFinalHandoffArtifact {
		return nil, errors.Join(err, errors.New("final worker artifact is not a bounded regular file"))
	}
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	raw, readErr := io.ReadAll(io.LimitReader(input, maximumFinalHandoffArtifact+1))
	after, statErr := input.Stat()
	closeErr := input.Close()
	if readErr != nil || statErr != nil || closeErr != nil || len(raw) > maximumFinalHandoffArtifact ||
		!os.SameFile(before, after) || before.Size() != after.Size() || before.ModTime() != after.ModTime() {
		return nil, errors.Join(readErr, statErr, closeErr, errors.New("final worker artifact changed while reading"))
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

func publishFinalArtifact(secret, directory, cell string, raw []byte) (finalRunnerArtifact, error) {
	relative := finalRunnerArtifactPath(directory, cell)
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
	if !filepath.IsAbs(root) || !filepath.IsAbs(secret) {
		return false
	}
	parent := filepath.Clean(filepath.Join(secret, "measurements", "workers"))
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
