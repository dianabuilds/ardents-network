package blockedentry

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/lab/sourceidentity"
)

const finalReceiptOutputLimit = 1 << 20

func inspectFinalImageReceipts(workspace string, config Config, sourceHash string) (
	finalProductReceipt, finalToolReceipt, error,
) {
	if err := verifyReceiptBaseLayers(config.LinuxImage, config.ProductImageID, config.ToolImageID); err != nil {
		return finalProductReceipt{}, finalToolReceipt{}, err
	}
	productLabels, err := boundedReceiptCommand("docker", "image", "inspect", "--format",
		"{{.Id}} {{index .Config.Labels \"io.ardents.stage5.target\"}} {{index .Config.Labels \"io.ardents.stage5.source.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.builder.image\"}} {{index .Config.Labels \"io.ardents.stage5.go.archive.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.go.recipe.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.go.module-cache.sha256\"}} {{index .Config.Labels \"org.opencontainers.image.base.digest\"}}",
		config.ProductImageID)
	lock, lockErr := loadFinalSupplyLock(workspace)
	wantProduct := config.ProductImageID + " product " + sourceHash + " " + config.GoBuilderImageID + " " +
		lock.GoArchiveSHA256 + " " + lock.GoRecipeSHA256 + " " + lock.GoModuleSHA256 + " " + finalImageHash
	if err != nil || lockErr != nil || strings.TrimSpace(string(productLabels)) != wantProduct {
		return finalProductReceipt{}, finalToolReceipt{}, errors.Join(err, lockErr,
			errors.New("product image receipt labels differ from the final source/profile"))
	}
	productRaw, err := readReceiptFiles(config.ProductImageID, "product",
		"cat /usr/share/ardents/stage5-source.sha256 /usr/share/ardents/go-archive.sha256 /usr/share/ardents/go-builder-recipe.sha256 /usr/share/ardents/go-module-cache.sha256; sha256sum /usr/local/bin/ardents-route /usr/local/bin/ardents-bridge /usr/local/bin/ardents-service /usr/local/bin/ardents-stream-app /usr/local/bin/ardents-publish-app /usr/local/bin/network-live.test /usr/local/bin/camouflage.test")
	if err != nil {
		return finalProductReceipt{}, finalToolReceipt{}, err
	}
	product, err := parseProductReceipt(string(productRaw), sourceHash, lock.GoArchiveSHA256,
		lock.GoRecipeSHA256, lock.GoModuleSHA256)
	if err != nil {
		return finalProductReceipt{}, finalToolReceipt{}, err
	}
	toolSource, err := sourceidentity.SourceSHA256(workspace)
	if err != nil {
		return finalProductReceipt{}, finalToolReceipt{}, err
	}
	toolLock, _, err := hashFile(filepath.Join(workspace, "lab", "carrier", "tools.lock"))
	if err != nil {
		return finalProductReceipt{}, finalToolReceipt{}, err
	}
	toolLabels, err := boundedReceiptCommand("docker", "image", "inspect", "--format",
		"{{.Id}} {{index .Config.Labels \"io.ardents.carrier-lab.target\"}} {{index .Config.Labels \"org.opencontainers.image.base.digest\"}}",
		config.ToolImageID)
	wantTool := config.ToolImageID + " tooling " + finalImageHash
	if err != nil || strings.TrimSpace(string(toolLabels)) != wantTool {
		return finalProductReceipt{}, finalToolReceipt{}, errors.Join(err,
			errors.New("tool image receipt labels differ from the accepted Carrier supply"))
	}
	toolRaw, err := readReceiptFiles(config.ToolImageID, "tool",
		"sha256sum /usr/share/ardents/carrier-lab-tools.lock /usr/local/bin/carrier-lab; cat /usr/share/ardents/carrier-lab-source.sha256")
	if err != nil {
		return finalProductReceipt{}, finalToolReceipt{}, err
	}
	tool, err := parseToolReceipt(string(toolRaw), toolLock, toolSource)
	return product, tool, err
}

func verifyReceiptBaseLayers(base string, images ...string) error {
	baseRaw, err := boundedReceiptCommand("docker", "image", "inspect", "--format", "{{json .RootFS.Layers}}", base)
	if err != nil {
		return err
	}
	var baseLayers []string
	if json.Unmarshal(bytes.TrimSpace(baseRaw), &baseLayers) != nil || len(baseLayers) == 0 {
		return errors.New("accepted Ubuntu image returned no rootfs identity")
	}
	for _, image := range images {
		raw, err := boundedReceiptCommand("docker", "image", "inspect", "--format", "{{json .RootFS.Layers}}", image)
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

func readReceiptFiles(image, role, script string) (result []byte, returnErr error) {
	token, err := receiptToken()
	if err != nil {
		return nil, err
	}
	name := "ardents-s5-receipt-" + role + "-" + token
	defer func() { returnErr = errors.Join(returnErr, removeOwnedReceiptContainer(name, token)) }()
	return boundedReceiptCommand("docker", "run", "--name", name,
		"--label", "io.ardents.stage5.receipt-owner="+token, "--pull", "never", "--network", "none",
		"--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true",
		"--entrypoint", "/bin/sh", image, "-ceu", script)
}

func receiptToken() (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func removeOwnedReceiptContainer(name, token string) error {
	label, err := boundedReceiptCommand("docker", "container", "inspect", "--format",
		"{{index .Config.Labels \"io.ardents.stage5.receipt-owner\"}}", name)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(label)) != token {
		return errors.New("refusing to remove an unowned image-receipt container")
	}
	_, err = boundedReceiptCommand("docker", "container", "rm", "--force", name)
	return err
}
