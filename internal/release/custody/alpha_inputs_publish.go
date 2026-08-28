package custody

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
)

func checkedAlphaOutput(path string) (string, error) {
	if path == "" || !filepath.IsAbs(path) {
		return "", ErrInvalid
	}
	output := filepath.Clean(path)
	parent := filepath.Dir(output)
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || output == parent {
		return "", ErrInvalid
	}
	if _, err := os.Lstat(output); err == nil {
		return "", ErrOutputExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("inspect alpha-input output: %w", err)
	}
	return output, nil
}

func publishAlphaInputFiles(ctx context.Context, output string, request alphaInputsRequest, files map[string][]byte,
	endpoint, control []byte, names []string, clock func() time.Time) ([]AlphaInputFile, [32]byte, error) {
	var zeroDigest [32]byte
	stage, err := os.MkdirTemp(filepath.Dir(output), ".ardents-alpha-inputs-")
	if err != nil {
		return nil, zeroDigest, fmt.Errorf("create alpha-input staging directory: %w", err)
	}
	defer os.RemoveAll(stage)
	if err := os.Chmod(stage, 0o700); err != nil {
		return nil, zeroDigest, fmt.Errorf("protect alpha-input staging directory: %w", err)
	}
	for _, name := range names {
		value, found := files[name]
		if !found || len(value) == 0 {
			return nil, zeroDigest, ErrInvalid
		}
		if err := writeAlphaInputFile(filepath.Join(stage, name), value, 0o600); err != nil {
			return nil, zeroDigest, err
		}
	}
	if err := writeAlphaInputFile(filepath.Join(stage, "ardents-linux-amd64"), endpoint, 0o700); err != nil {
		return nil, zeroDigest, err
	}
	if err := writeAlphaInputFile(filepath.Join(stage, "ardents-control-linux-amd64"), control, 0o700); err != nil {
		return nil, zeroDigest, err
	}
	manifest, err := alphaInputManifest(stage)
	if err != nil {
		return nil, zeroDigest, err
	}
	if err := writeAlphaInputFile(filepath.Join(stage, "SHA256SUMS"), manifest, 0o600); err != nil {
		return nil, zeroDigest, err
	}
	if err := preflightAlphaBundle(ctx, stage, request, manifest); err != nil {
		return nil, zeroDigest, err
	}
	for _, name := range []string{"ardents-linux-amd64", "ardents-control-linux-amd64", "SHA256SUMS"} {
		if err := os.Remove(filepath.Join(stage, name)); err != nil {
			return nil, zeroDigest, fmt.Errorf("remove preflight-only file: %w", err)
		}
	}
	receipt, outputDigest, err := verifyAlphaInputStage(stage, files, names)
	if err != nil {
		return nil, zeroDigest, err
	}
	if err := ctx.Err(); err != nil {
		return nil, zeroDigest, err
	}
	if !alphaInputsFresh(request, clock().UTC()) {
		return nil, zeroDigest, fmt.Errorf("%w: request expired during construction", ErrPreflight)
	}
	if _, err := os.Lstat(output); err == nil {
		return nil, zeroDigest, ErrOutputExists
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, zeroDigest, fmt.Errorf("recheck alpha-input output: %w", err)
	}
	if err := os.Rename(stage, output); err != nil {
		if _, existsErr := os.Lstat(output); existsErr == nil {
			return nil, zeroDigest, ErrOutputExists
		}
		return nil, zeroDigest, fmt.Errorf("publish alpha-input directory: %w", err)
	}
	return receipt, outputDigest, nil
}

func writeAlphaInputFile(path string, value []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return fmt.Errorf("create alpha-input staging file: %w", err)
	}
	if err := file.Chmod(mode); err != nil {
		file.Close()
		return fmt.Errorf("protect alpha-input staging file: %w", err)
	}
	if _, err := file.Write(value); err != nil {
		file.Close()
		return fmt.Errorf("write alpha-input staging file: %w", err)
	}
	if err := file.Sync(); err != nil {
		file.Close()
		return fmt.Errorf("flush alpha-input staging file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close alpha-input staging file: %w", err)
	}
	return nil
}

func alphaInputManifest(root string) ([]byte, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	var manifest strings.Builder
	for _, entry := range entries {
		if !entry.Type().IsRegular() || entry.Type()&os.ModeSymlink != 0 || entry.Name() == "SHA256SUMS" {
			return nil, ErrInvalid
		}
		value, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(value)
		fmt.Fprintf(&manifest, "%x  %s\n", digest, entry.Name())
	}
	return []byte(manifest.String()), nil
}

func preflightAlphaBundle(ctx context.Context, bundle string, request alphaInputsRequest, manifest []byte) error {
	inspectionRoot, err := os.MkdirTemp("", "ardents-alpha-control-preflight-")
	if err != nil {
		return fmt.Errorf("create alpha control preflight root: %w", err)
	}
	defer os.RemoveAll(inspectionRoot)
	pin := sha256.Sum256(manifest)
	report, err := inspection.Inspect(ctx, inspection.Config{Root: inspectionRoot, At: request.ReferenceTime,
		Enrollment: enrollment.Request{BundleRoot: bundle, ExecutablePath: filepath.Join(bundle, "ardents-linux-amd64"),
			Pin:         enrollment.Pin{Cohort: request.Cohort, Release: request.Release, Platform: "linux-amd64", ManifestSHA256: hex.EncodeToString(pin[:])},
			Environment: request.Environment, Network: request.Network, TargetPath: alphaEndpointTargetPath,
			Architecture: "amd64", ReferenceTime: request.ReferenceTime}})
	if err != nil {
		return fmt.Errorf("%w: complete control inspection: %v", ErrPreflight, err)
	}
	if report.Release != "release-accepted" || report.NetworkID != request.NetworkState.NetworkID ||
		report.NetworkDigest != request.NetworkState.EpochDigest || report.Inspection.Catalog != alphacontrol.OutcomeAccepted {
		return fmt.Errorf("%w: complete control identity mismatch", ErrPreflight)
	}
	for _, component := range report.Inspection.Components {
		if component.Outcome != alphacontrol.OutcomeAccepted {
			return fmt.Errorf("%w: component %d is %s", ErrPreflight, component.Class, component.Outcome)
		}
	}
	return nil
}

func verifyAlphaInputStage(root string, expected map[string][]byte, names []string) ([]AlphaInputFile, [32]byte, error) {
	var outputDigest [32]byte
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != len(names) {
		return nil, outputDigest, ErrInvalid
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	receipt := make([]AlphaInputFile, 0, len(entries))
	for _, entry := range entries {
		if _, found := allowed[entry.Name()]; !found || !entry.Type().IsRegular() || entry.Type()&os.ModeSymlink != 0 {
			return nil, outputDigest, ErrInvalid
		}
		value, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil || !equalAlphaBytes(value, expected[entry.Name()]) {
			return nil, outputDigest, ErrInvalid
		}
		receipt = append(receipt, AlphaInputFile{Name: entry.Name(), Size: int64(len(value)), Digest: sha256.Sum256(value)})
	}
	sort.Slice(receipt, func(left, right int) bool { return receipt[left].Name < receipt[right].Name })
	hash := sha256.New()
	for _, file := range receipt {
		fmt.Fprintf(hash, "%s\x00%d\x00%x\n", file.Name, file.Size, file.Digest)
	}
	copy(outputDigest[:], hash.Sum(nil))
	return receipt, outputDigest, nil
}

func equalAlphaBytes(left, right []byte) bool {
	return len(left) == len(right) && sha256.Sum256(left) == sha256.Sum256(right)
}
