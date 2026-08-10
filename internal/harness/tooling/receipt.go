package tooling

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/experimentidentity"
)

const toolingBuildReceiptSchema = "carrier-lab-tooling-build-receipt/v1"

type toolingBuildReceipt struct {
	SchemaVersion    string `json:"schema_version"`
	ImageID          string `json:"image_id"`
	BaseImage        string `json:"base_image"`
	ToolLockSHA256   string `json:"tool_lock_sha256"`
	SourceSHA256     string `json:"source_sha256"`
	CarrierLabSHA256 string `json:"carrier_lab_sha256"`
}

type toolingImageInspect struct {
	ID     string `json:"Id"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

func inspectToolingBuildReceipt(ctx context.Context, layout runLayout, project, image string) (receipt toolingBuildReceipt, returnErr error) {
	identity, err := readToolLock(carrierLabToolLockPath(layout.repositoryRoot))
	if err != nil {
		return toolingBuildReceipt{}, err
	}
	inspectionData, err := exec.CommandContext(ctx, "docker", "image", "inspect", image).Output()
	if err != nil {
		return toolingBuildReceipt{}, fmt.Errorf("inspect tooling image: %w", err)
	}
	var inspections []toolingImageInspect
	if err := json.Unmarshal(inspectionData, &inspections); err != nil || len(inspections) != 1 {
		return toolingBuildReceipt{}, errors.New("cannot decode tooling image inspection")
	}
	inspection := inspections[0]
	baseName, baseDigest, found := strings.Cut(identity.BaseImage, "@")
	if !found || inspection.ID != image ||
		inspection.Config.Labels["org.opencontainers.image.base.name"] != baseName ||
		inspection.Config.Labels["org.opencontainers.image.base.digest"] != baseDigest ||
		inspection.Config.Labels["io.ardents.carrier-lab.target"] != "tooling" {
		return toolingBuildReceipt{}, errors.New("tooling image identity labels do not match the locked build contract")
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
		"sha256sum /usr/share/ardents/carrier-lab-tools.lock /usr/local/bin/carrier-lab; cat /usr/share/ardents/carrier-lab-source.sha256")
	output, err := command.CombinedOutput()
	if err != nil {
		return toolingBuildReceipt{}, fmt.Errorf("read tooling image identities: %w: %s", err, strings.TrimSpace(string(output)))
	}
	embeddedLock, binary, source, err := parseToolingImageIdentities(string(output))
	if err != nil {
		return toolingBuildReceipt{}, err
	}
	currentSource, err := experimentidentity.SourceSHA256(layout.repositoryRoot)
	if err != nil {
		return toolingBuildReceipt{}, err
	}
	if embeddedLock != identity.LockSHA256 {
		return toolingBuildReceipt{}, errors.New("tooling image embeds a different tool lock")
	}
	if source != currentSource {
		return toolingBuildReceipt{}, errors.New("tooling image was not built from the current Carrier Lab source snapshot")
	}
	return toolingBuildReceipt{
		SchemaVersion: toolingBuildReceiptSchema,
		ImageID:       image, BaseImage: identity.BaseImage, ToolLockSHA256: identity.LockSHA256,
		SourceSHA256: source, CarrierLabSHA256: binary,
	}, nil
}

func removeOwnedReceiptContainer(ctx context.Context, containerName, project string) error {
	output, err := exec.CommandContext(ctx, "docker", "ps", "--all", "--quiet",
		"--filter", "name=^/"+containerName+"$", "--filter", "label=com.docker.compose.project="+project).CombinedOutput()
	if err != nil {
		return fmt.Errorf("inspect owned receipt container: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var cleanupErr error
	for _, containerID := range strings.Fields(string(output)) {
		removeOutput, err := exec.CommandContext(ctx, "docker", "container", "rm", "--force", containerID).CombinedOutput()
		if err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("remove owned receipt container: %w: %s", err, strings.TrimSpace(string(removeOutput))))
		}
	}
	return cleanupErr
}

func parseToolingImageIdentities(output string) (lock, binary, source string, err error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		return "", "", "", errors.New("tooling image returned an incomplete identity receipt")
	}
	readFileHash := func(line, expectedPath string) (string, error) {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 || fields[1] != expectedPath || !validSHA256String(fields[0]) {
			return "", errors.New("tooling image returned a malformed file identity")
		}
		return fields[0], nil
	}
	lock, err = readFileHash(lines[0], "/usr/share/ardents/carrier-lab-tools.lock")
	if err != nil {
		return "", "", "", err
	}
	binary, err = readFileHash(lines[1], "/usr/local/bin/carrier-lab")
	if err != nil {
		return "", "", "", err
	}
	source = strings.TrimSpace(lines[2])
	if !validSHA256String(source) {
		return "", "", "", errors.New("tooling image returned a malformed source identity")
	}
	return lock, binary, source, nil
}
