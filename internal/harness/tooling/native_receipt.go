package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const nativeImageReceiptSchema = "carrier-lab-native-image-receipt/v1"

// NativeImageReceipt binds both runnable native-lab images to the reviewed
// base, tool lock, current qualification source, and one identical binary.
type NativeImageReceipt struct {
	SchemaVersion      string `json:"schema_version"`
	ApplicationImageID string `json:"application_image_id"`
	ToolImageID        string `json:"tool_image_id"`
	BaseImage          string `json:"base_image"`
	ToolLockSHA256     string `json:"tool_lock_sha256"`
	SourceSHA256       string `json:"source_sha256"`
	CarrierLabSHA256   string `json:"carrier_lab_sha256"`
}

// VerifyNativeImages rejects runnable images that are not tied to the current
// verified Carrier Lab source and exact laboratory tool supply.
func VerifyNativeImages(ctx context.Context, repositoryRoot, project, applicationImage, toolImage string) (NativeImageReceipt, error) {
	layout := runLayout{repositoryRoot: repositoryRoot}
	toolReceipt, err := inspectToolingBuildReceipt(ctx, layout, project, toolImage)
	if err != nil {
		return NativeImageReceipt{}, err
	}
	applicationBinary, applicationSource, err := inspectApplicationImage(ctx, project, applicationImage, toolReceipt.BaseImage)
	if err != nil {
		return NativeImageReceipt{}, err
	}
	if applicationSource != toolReceipt.SourceSHA256 || applicationBinary != toolReceipt.CarrierLabSHA256 {
		return NativeImageReceipt{}, errors.New("native application and tooling images are not one verified build pair")
	}
	return NativeImageReceipt{
		SchemaVersion: nativeImageReceiptSchema, ApplicationImageID: applicationImage, ToolImageID: toolImage,
		BaseImage: toolReceipt.BaseImage, ToolLockSHA256: toolReceipt.ToolLockSHA256,
		SourceSHA256: toolReceipt.SourceSHA256, CarrierLabSHA256: toolReceipt.CarrierLabSHA256,
	}, nil
}

func inspectApplicationImage(ctx context.Context, project, image, baseImage string) (binary, source string, returnErr error) {
	data, err := exec.CommandContext(ctx, "docker", "image", "inspect", image).Output()
	if err != nil {
		return "", "", fmt.Errorf("inspect application image: %w", err)
	}
	var inspections []toolingImageInspect
	if err := json.Unmarshal(data, &inspections); err != nil || len(inspections) != 1 {
		return "", "", errors.New("cannot decode application image inspection")
	}
	baseName, baseDigest, found := strings.Cut(baseImage, "@")
	labels := inspections[0].Config.Labels
	if !found || inspections[0].ID != image || labels["org.opencontainers.image.base.name"] != baseName ||
		labels["org.opencontainers.image.base.digest"] != baseDigest || labels["io.ardents.carrier-lab.target"] != "application" {
		return "", "", errors.New("application image identity labels do not match the locked build contract")
	}
	containerName := project + "-receipt"
	defer func() {
		cleanupContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		returnErr = errors.Join(returnErr, removeOwnedReceiptContainer(cleanupContext, containerName, project))
	}()
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--name", containerName,
		"--label", "com.docker.compose.project="+project, "--pull", "never", "--network", "none", "--read-only",
		"--cap-drop", "ALL", "--security-opt", "no-new-privileges",
		"--entrypoint", "/bin/sh", image, "-ceu",
		"sha256sum /usr/local/bin/carrier-lab; cat /usr/share/ardents/carrier-lab-source.sha256")
	output, err := command.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("read application image identities: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return parseApplicationImageIdentities(string(output))
}

func parseApplicationImageIdentities(output string) (binary, source string, err error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		return "", "", errors.New("application image returned an incomplete identity receipt")
	}
	fields := strings.Fields(strings.TrimSpace(lines[0]))
	if len(fields) != 2 || fields[1] != "/usr/local/bin/carrier-lab" || !validSHA256String(fields[0]) {
		return "", "", errors.New("application image returned a malformed binary identity")
	}
	source = strings.TrimSpace(lines[1])
	if !validSHA256String(source) {
		return "", "", errors.New("application image returned a malformed source identity")
	}
	return fields[0], source, nil
}
