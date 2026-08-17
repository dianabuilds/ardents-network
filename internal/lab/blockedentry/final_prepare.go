package blockedentry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func prepareFinal(config Config) (result Result, returnErr error) {
	workspace, workspaceErr := filepath.Abs(config.WorkspaceRoot)
	output, outputErr := filepath.Abs(config.PreparationRoot)
	configuration, configurationErr := filepath.Abs(config.ConfigurationRoot)
	if workspaceErr != nil || outputErr != nil || configurationErr != nil {
		return Result{}, errors.Join(workspaceErr, outputErr, configurationErr)
	}
	if err := validateWorkspace(workspace); err != nil {
		return Result{}, err
	}
	outputAliased, outputAliasErr := pathHasSymlink(filepath.Dir(output))
	configurationAliased, configurationAliasErr := pathHasSymlink(configuration)
	if outputAliasErr != nil || configurationAliasErr != nil {
		return Result{}, errors.Join(outputAliasErr, configurationAliasErr)
	}
	if outputAliased || configurationAliased || within(workspace, output) || workspace == output ||
		within(workspace, configuration) || workspace == configuration || within(output, configuration) ||
		within(configuration, output) || output == configuration {
		return Result{}, errors.New("final preparation and configuration roots must be external, disjoint, and unaliased")
	}
	if _, err := os.Lstat(output); !os.IsNotExist(err) {
		return Result{}, errors.New("final campaign preparation root must not already exist")
	}
	if config.LinuxImage != finalLinuxImage || config.ImageSHA256 != finalImageHash || config.Kernel == "" ||
		!imageID(config.GoBuilderImageID) || !imageID(config.ToolImageID) ||
		config.GoBuilderImageID == config.ToolImageID || config.ProductImageID != "" {
		return Result{}, errors.New("final image or kernel identity differs from the accepted profile")
	}
	if err := validateFinalConfigurationTree(configuration); err != nil {
		return Result{}, err
	}
	commit, sourceHash, sourceRoot, temporarySource, err := materializeCommittedSource(workspace)
	if err != nil {
		return Result{}, err
	}
	var productCleanup func() error
	defer func() {
		sourceCleanupErr := os.RemoveAll(temporarySource)
		if returnErr != nil || sourceCleanupErr != nil {
			if productCleanup != nil {
				returnErr = errors.Join(returnErr, sourceCleanupErr, productCleanup())
				return
			}
		}
		returnErr = errors.Join(returnErr, sourceCleanupErr)
	}()
	supplyLock, err := loadFinalSupplyLock(sourceRoot)
	if err != nil {
		return Result{}, err
	}
	if config.GoBuilderImageID != supplyLock.GoBuilderImageID || config.ToolImageID != supplyLock.ToolImageID {
		return Result{}, errors.New("final image arguments differ from the accepted supply lock")
	}
	archivePath := filepath.Join(temporarySource, "source.tar")
	config.ProductImageID, productCleanup, err = buildFinalProductImage(archivePath, sourceHash, config, supplyLock)
	if err != nil {
		return Result{}, err
	}
	productReceipt, toolReceipt, err := inspectFinalImageReceipts(sourceRoot, config, sourceHash)
	if err != nil {
		return Result{}, err
	}
	if toolReceipt.CarrierSHA256 != supplyLock.CarrierLabSHA256 {
		return Result{}, errors.New("carrier binary differs from the accepted supply lock")
	}
	clientHash, _, clientErr := hashFile(config.ClientPath)
	serverHash, _, serverErr := hashFile(config.ServerPath)
	if clientErr != nil || serverErr != nil {
		return Result{}, errors.Join(clientErr, serverErr)
	}
	if clientHash != finalClientHash || serverHash != finalServerHash {
		return Result{}, errors.New("final WebTunnel binary identity differs from R-036")
	}
	if err := os.Mkdir(output, 0o700); err != nil {
		return Result{}, err
	}
	complete := false
	defer func() {
		if !complete {
			returnErr = errors.Join(returnErr, os.RemoveAll(output))
		}
	}()
	runnerPath, err := exportFinalRunner(config.ProductImageID, output, productReceipt.NetworkSHA256)
	if err != nil {
		return Result{}, err
	}
	configurations, err := freezePreparationConfigurations(configuration, output)
	if err != nil {
		return Result{}, err
	}
	runtimeCompose, err := freezeRuntimeCompose(sourceRoot, output)
	if err != nil {
		return Result{}, err
	}
	supplyLockCommitment, err := freezeFinalSupplyLock(sourceRoot, output)
	if err != nil {
		return Result{}, err
	}
	value := exactFinalSpec(commit, sourceHash, config, clientHash, serverHash, configurations, runtimeCompose,
		supplyLockCommitment, productReceipt, toolReceipt)
	for range len(value.CellOrder) {
		seed := make([]byte, 32)
		if _, err := rand.Read(seed); err != nil {
			return Result{}, err
		}
		value.Seeds = append(value.Seeds, hex.EncodeToString(seed))
	}
	path := filepath.Join(output, "final-spec.json")
	if err := writeJSON(path, value); err != nil {
		return Result{}, err
	}
	if err := os.Chmod(path, 0o400); err != nil {
		return Result{}, err
	}
	if err := protectEvidenceTree(output); err != nil {
		return Result{}, err
	}
	complete = true
	return Result{SpecPath: path, RunnerPath: runnerPath}, nil
}

func hexDigest(value string, bytes int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == bytes
}

func imageID(value string) bool {
	return strings.HasPrefix(value, "sha256:") && hexDigest(strings.TrimPrefix(value, "sha256:"), 32)
}
