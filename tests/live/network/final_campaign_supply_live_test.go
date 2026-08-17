//go:build live

package network_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func validFinalRunnerSupplyIdentity(value finalRunnerSchedule) bool {
	return value.Schema == "ardents-h3-s5-final-spec-v1" && finalHex(value.RepositoryCommit, 20) &&
		finalHex(value.SourceSHA256, 32) && value.LinuxImage == finalRunnerLinuxImage &&
		value.ImageSHA256 == strings.TrimPrefix(finalRunnerLinuxImage, "ubuntu@sha256:") && value.Kernel != "" &&
		finalImageID(value.ProductImageID) && finalImageID(value.ToolImageID) &&
		finalImageID(value.GoBuilderImageID) && value.ProductImageID != value.ToolImageID &&
		value.ProductImageID != value.GoBuilderImageID && value.ToolImageID != value.GoBuilderImageID &&
		value.GoBuilderVersion == "go version go1.26.6 linux/amd64" &&
		value.SupplyLock.Path == "runtime/supply.lock.json" &&
		finalHex(value.SupplyLock.SHA256, 32) && value.SupplyLock.Bytes > 0 &&
		finalHex(value.ClientSHA256, 32) &&
		value.RuntimeCompose.Path == "runtime/blocked-entry.compose.yaml" &&
		finalHex(value.RuntimeCompose.SHA256, 32) && value.RuntimeCompose.Bytes > 0 &&
		validRunnerProductReceipt(value.ProductReceipt, value.SourceSHA256) &&
		validRunnerToolReceipt(value.ToolReceipt) &&
		finalHex(value.ServerSHA256, 32) && len(value.Endpoint) > 0 && len(value.ReferenceBridge) > 0 &&
		len(value.StrongerBridge) > 0 && len(value.Collector) > 0 && len(value.Network) > 0 &&
		len(value.Clocks) > 0 && len(value.Configurations) > 0
}

func validRunnerProductReceipt(value finalRunnerProductReceipt, source string) bool {
	return value.SourceSHA256 == source && finalHex(value.GoArchiveSHA256, 32) &&
		finalHex(value.GoRecipeSHA256, 32) && finalHex(value.GoModuleSHA256, 32) &&
		finalHex(value.RouteSHA256, 32) &&
		finalHex(value.BridgeSHA256, 32) && finalHex(value.ServiceSHA256, 32) &&
		finalHex(value.StreamSHA256, 32) && finalHex(value.PublishSHA256, 32) &&
		finalHex(value.NetworkSHA256, 32) && finalHex(value.AdapterSHA256, 32)
}

func validRunnerToolReceipt(value finalRunnerToolReceipt) bool {
	return value.BaseDigest == strings.TrimPrefix(finalRunnerLinuxImage, "ubuntu@") &&
		finalHex(value.ToolLockSHA256, 32) && finalHex(value.SourceSHA256, 32) &&
		finalHex(value.CarrierSHA256, 32)
}

func verifyFinalRuntimeCompose(value finalRunnerSchedule) error {
	path := os.Getenv("ARDENTS_BLOCKED_COMPOSE_FILE")
	pathInfo, err := os.Lstat(path)
	if err != nil || pathInfo.Mode()&os.ModeSymlink != 0 || !pathInfo.Mode().IsRegular() ||
		pathInfo.Size() != value.RuntimeCompose.Bytes {
		return errors.Join(err, errors.New("frozen runtime Compose artifact is unavailable"))
	}
	input, err := os.Open(path)
	if err != nil {
		return err
	}
	digest := sha256.New()
	written, copyErr := io.Copy(digest, io.LimitReader(input, (16<<20)+1))
	info, statErr := input.Stat()
	closeErr := input.Close()
	if copyErr != nil || statErr != nil || closeErr != nil || !os.SameFile(pathInfo, info) ||
		written != value.RuntimeCompose.Bytes || hex.EncodeToString(digest.Sum(nil)) != value.RuntimeCompose.SHA256 {
		return errors.Join(copyErr, statErr, closeErr, errors.New("frozen runtime Compose artifact changed"))
	}
	return nil
}

const finalRunnerLinuxImage = "ubuntu@sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"

func verifyFinalRunnerSupply(value finalRunnerSchedule, projectToken string) error {
	root, err := exactFinalWorkspace(os.Getenv("ARDENTS_BLOCKED_WORKSPACE_ROOT"))
	if err != nil {
		return err
	}
	if _, err := finalSupplyOutput("git", "-C", root, "diff", "--quiet", "HEAD", "--"); err != nil {
		return errors.New("final worker source has tracked modifications")
	}
	commit, err := finalSupplyOutput("git", "-C", root, "rev-parse", "HEAD")
	if err != nil || strings.TrimSpace(string(commit)) != value.RepositoryCommit {
		return errors.New("final worker commit differs from the frozen specification")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	archive := exec.CommandContext(ctx, "git", "-C", root, "archive", "--format=tar", "HEAD")
	prepareFinalProcess(archive)
	archive.WaitDelay = 5 * time.Second
	archive.Cancel = func() error { return terminateFinalProcess(archive) }
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return err
	}
	if err := archive.Start(); err != nil {
		return err
	}
	digest := sha256.New()
	written, overflow, copyErr := copyFinalArchive(digest, pipe, 1<<31)
	if overflow || copyErr != nil {
		_ = terminateFinalProcess(archive)
	}
	waitErr := archive.Wait()
	if ctx.Err() != nil || written == 0 || overflow || copyErr != nil || waitErr != nil ||
		hex.EncodeToString(digest.Sum(nil)) != value.SourceSHA256 {
		return errors.Join(copyErr, waitErr, errors.New("final worker source archive differs from the frozen specification"))
	}
	if err := verifyFinalRunnerSupplyLock(root, value); err != nil {
		return err
	}
	if err := verifyFinalImageIDs(value.ProductImageID, value.ToolImageID); err != nil {
		return err
	}
	if err := verifyFinalBaseLayers(value.LinuxImage, value.ProductImageID, value.ToolImageID); err != nil {
		return err
	}
	if err := verifyFinalImageLabels(value.ProductImageID, value.ToolImageID, value.GoBuilderImageID,
		value.SourceSHA256, value.ProductReceipt.GoArchiveSHA256, value.ProductReceipt.GoRecipeSHA256,
		value.ProductReceipt.GoModuleSHA256); err != nil {
		return err
	}
	if err := verifyFinalEmbeddedReceipts(value, projectToken); err != nil {
		return err
	}
	return verifyFinalRuntimeCompose(value)
}

func copyFinalArchive(output io.Writer, input io.Reader, limit int64) (int64, bool, error) {
	written, err := io.Copy(output, io.LimitReader(input, limit+1))
	return written, written > limit, err
}

func verifyFinalBaseLayers(base string, images ...string) error {
	baseRaw, err := finalSupplyOutput("docker", "image", "inspect", "--format", "{{json .RootFS.Layers}}", base)
	if err != nil {
		return err
	}
	var baseLayers []string
	if json.Unmarshal(bytes.TrimSpace(baseRaw), &baseLayers) != nil || len(baseLayers) == 0 {
		return errors.New("accepted Ubuntu image returned no rootfs identity")
	}
	for _, image := range images {
		raw, err := finalSupplyOutput("docker", "image", "inspect", "--format", "{{json .RootFS.Layers}}", image)
		var layers []string
		if err != nil || json.Unmarshal(bytes.TrimSpace(raw), &layers) != nil || len(layers) <= len(baseLayers) {
			return errors.Join(err, errors.New("final image returned no derived rootfs identity"))
		}
		for index := range baseLayers {
			if layers[index] != baseLayers[index] {
				return errors.New("final image is not derived from the accepted Ubuntu rootfs")
			}
		}
	}
	return nil
}

func exactFinalWorkspace(root string) (string, error) {
	absolute, err := filepath.Abs(root)
	if err != nil || root == "" {
		return "", errors.Join(err, errors.New("final worker workspace is unavailable"))
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil || filepath.Clean(resolved) != filepath.Clean(absolute) {
		return "", errors.Join(err, errors.New("final worker workspace is aliased"))
	}
	return absolute, nil
}

func verifyFinalImageIDs(images ...string) error {
	arguments := append([]string{"image", "inspect", "--format", "{{.Id}}"}, images...)
	output, err := finalSupplyOutput("docker", arguments...)
	if err != nil {
		return fmt.Errorf("inspect frozen final images: %w", err)
	}
	observed := strings.Fields(string(output))
	if len(observed) != len(images) {
		return errors.New("frozen final image inspection is incomplete")
	}
	for index := range images {
		if observed[index] != images[index] {
			return errors.New("preloaded final image identity changed")
		}
	}
	return nil
}

func verifyFinalImageLabels(product, tool, builder, source, archive, recipe, modules string) error {
	productOutput, productErr := finalSupplyOutput("docker", "image", "inspect", "--format",
		"{{index .Config.Labels \"io.ardents.stage5.target\"}} {{index .Config.Labels \"io.ardents.stage5.source.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.builder.image\"}} {{index .Config.Labels \"io.ardents.stage5.go.archive.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.go.recipe.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.go.module-cache.sha256\"}} {{index .Config.Labels \"org.opencontainers.image.base.digest\"}}",
		product)
	toolOutput, toolErr := finalSupplyOutput("docker", "image", "inspect", "--format",
		"{{index .Config.Labels \"io.ardents.carrier-lab.target\"}}", tool)
	wantProduct := "product " + source + " " + builder + " " +
		archive + " " + recipe + " " + modules +
		" sha256:7b202b0e2e0028c6250f5fcf41d04df492d145a1654c6995a6553f0c1f6f1960"
	if productErr != nil || toolErr != nil || strings.TrimSpace(string(productOutput)) != wantProduct ||
		strings.TrimSpace(string(toolOutput)) != "tooling" {
		return errors.Join(productErr, toolErr, errors.New("final image labels differ from the frozen supply contract"))
	}
	return nil
}

func finalSupplyOutput(name string, arguments ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, name, arguments...)
	stdout, stderr, err := runFinalBoundedProcess(command, 1<<20)
	if ctx.Err() != nil {
		return nil, errors.New("final supply command exceeded its time bound")
	}
	if err != nil {
		return nil, fmt.Errorf("final supply command failed: %w: %s", err, strings.TrimSpace(string(stderr)))
	}
	return stdout, nil
}

func finalHex(value string, bytes int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == bytes
}

func finalImageID(value string) bool {
	return strings.HasPrefix(value, "sha256:") && finalHex(strings.TrimPrefix(value, "sha256:"), 32)
}

func finalProductImage(t *testing.T, development string) (string, bool) {
	t.Helper()
	if os.Getenv("ARDENTS_BLOCKED_CELL_WORKER") != "1" {
		return development, true
	}
	image := os.Getenv("ARDENTS_FINAL_PRODUCT_IMAGE")
	if !finalImageID(image) {
		t.Fatal("final worker product image is not content-addressed")
	}
	if err := verifyFinalImageIDs(image); err != nil {
		t.Fatal(err)
	}
	return image, false
}
