package blockedentry

import (
	"errors"
	"os"
	"strings"
	"time"
)

const finalBuildOutputLimit = 16 << 20
const finalGoBuilderVersion = "go version go1.26.6 linux/amd64"

func buildFinalProductImage(archivePath, sourceHash string, config Config,
	lock finalSupplyLock,
) (string, func() error, error) {
	if !imageID(config.GoBuilderImageID) || config.GoBuilderImageID == config.ToolImageID {
		return "", nil, errors.New("final Go builder image identity is invalid")
	}
	observed, err := boundedReceiptCommand("docker", "image", "inspect", "--format", "{{.Id}}",
		config.GoBuilderImageID)
	if err != nil || strings.TrimSpace(string(observed)) != config.GoBuilderImageID {
		return "", nil, errors.Join(err, errors.New("final Go builder image is not preloaded by exact ID"))
	}
	if err := verifyReceiptBaseLayers(config.LinuxImage, config.GoBuilderImageID); err != nil {
		return "", nil, err
	}
	labels, labelErr := boundedReceiptCommand("docker", "image", "inspect", "--format",
		"{{index .Config.Labels \"io.ardents.stage5.target\"}} {{index .Config.Labels \"io.ardents.stage5.go.archive.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.go.recipe.sha256\"}} {{index .Config.Labels \"io.ardents.stage5.go.module-cache.sha256\"}}",
		config.GoBuilderImageID)
	wantLabels := "go-builder " + lock.GoArchiveSHA256 + " " + lock.GoRecipeSHA256 + " " + lock.GoModuleSHA256
	receipt, err := readReceiptFiles(config.GoBuilderImageID, "builder",
		"/usr/local/go/bin/go version; cat /usr/share/ardents/go-archive.sha256 /usr/share/ardents/go-builder-recipe.sha256 /usr/share/ardents/go-module-cache.sha256")
	wantReceipt := finalGoBuilderVersion + "\n" + lock.GoArchiveSHA256 + "\n" + lock.GoRecipeSHA256 + "\n" + lock.GoModuleSHA256
	if labelErr != nil || strings.TrimSpace(string(labels)) != wantLabels ||
		err != nil || strings.TrimSpace(string(receipt)) != wantReceipt {
		return "", nil, errors.Join(labelErr, err,
			errors.New("final Go builder toolchain differs from its accepted recipe and archive"))
	}
	token, err := receiptToken()
	if err != nil {
		return "", nil, err
	}
	tag := "ardents-stage5-product:" + token
	builderTag := "ardents-stage5-builder:" + token
	if _, err := boundedReceiptCommand("docker", "image", "inspect", tag); err == nil {
		return "", nil, errors.New("random final product image tag already exists")
	}
	if _, err := boundedReceiptCommand("docker", "image", "inspect", builderTag); err == nil {
		return "", nil, errors.New("random final builder alias already exists")
	}
	if _, err := boundedReceiptCommand("docker", "image", "tag", config.GoBuilderImageID, builderTag); err != nil {
		return "", nil, err
	}
	archive, err := os.Open(archivePath)
	if err != nil {
		return "", nil, errors.Join(err, removeOwnedBuilderAlias(builderTag, config.GoBuilderImageID))
	}
	_, buildErr := boundedSupplyCommandInput(30*time.Minute, finalBuildOutputLimit, archive, "docker", "build", "--pull=false",
		"--no-cache", "--network", "none", "--build-arg", "ARDENTS_SOURCE_SHA256="+sourceHash,
		"--build-arg", "ARDENTS_GO_BUILDER_IMAGE="+builderTag,
		"--build-arg", "ARDENTS_GO_BUILDER_ID="+config.GoBuilderImageID,
		"--build-arg", "ARDENTS_GO_ARCHIVE_SHA256="+lock.GoArchiveSHA256,
		"--build-arg", "ARDENTS_GO_RECIPE_SHA256="+lock.GoRecipeSHA256,
		"--build-arg", "ARDENTS_GO_MODULE_SHA256="+lock.GoModuleSHA256,
		"--label", "io.ardents.stage5.build-owner="+token, "--file",
		"tests/live/blocked-entry.Dockerfile", "--tag", tag, "-")
	closeErr := archive.Close()
	aliasCleanupErr := removeOwnedBuilderAlias(builderTag, config.GoBuilderImageID)
	if buildErr != nil || closeErr != nil || aliasCleanupErr != nil {
		closeErr = errors.Join(closeErr, removeOwnedProductImageIfOwned(tag, token))
		return "", nil, errors.Join(buildErr, closeErr, aliasCleanupErr)
	}
	identity, err := boundedReceiptCommand("docker", "image", "inspect", "--format", "{{.Id}}", tag)
	result := strings.TrimSpace(string(identity))
	if err != nil || !imageID(result) {
		cleanupErr := removeOwnedProductImage(tag, token)
		return "", nil, errors.Join(err, cleanupErr,
			errors.New("built final product image has no content identity"))
	}
	cleanup := func() error { return removeOwnedProductImage(tag, token) }
	return result, cleanup, nil
}

func removeOwnedProductImageIfOwned(tag, token string) error {
	identity, err := boundedReceiptCommand("docker", "image", "ls", "--no-trunc", "--quiet", tag)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(identity)) == "" {
		return nil
	}
	return removeOwnedProductImage(tag, token)
}

func removeOwnedProductImage(tag, token string) error {
	label, err := boundedReceiptCommand("docker", "image", "inspect", "--format",
		"{{index .Config.Labels \"io.ardents.stage5.build-owner\"}}", tag)
	if err != nil || strings.TrimSpace(string(label)) != token {
		return errors.Join(err, errors.New("refusing to remove an unowned final product image"))
	}
	_, err = boundedReceiptCommand("docker", "image", "rm", tag)
	return err
}

func removeOwnedBuilderAlias(tag, identity string) error {
	observed, err := boundedReceiptCommand("docker", "image", "inspect", "--format", "{{.Id}}", tag)
	if err != nil || strings.TrimSpace(string(observed)) != identity {
		return errors.Join(err, errors.New("refusing to remove an unowned final builder alias"))
	}
	_, err = boundedReceiptCommand("docker", "image", "rm", tag)
	return err
}
