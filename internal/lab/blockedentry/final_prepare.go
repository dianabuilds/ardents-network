package blockedentry

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func prepareFinal(config Config) (Result, error) {
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
	if config.LinuxImage != finalLinuxImage || config.ImageSHA256 != finalImageHash || config.Kernel == "" {
		return Result{}, errors.New("final image or kernel identity differs from the accepted profile")
	}
	if err := validateFinalConfigurationTree(configuration); err != nil {
		return Result{}, err
	}
	commit, sourceHash, err := committedSourceIdentity(workspace)
	if err != nil {
		return Result{}, err
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
			_ = os.RemoveAll(output)
		}
	}()
	configurations, err := freezePreparationConfigurations(configuration, output)
	if err != nil {
		return Result{}, err
	}
	value := exactFinalSpec(commit, sourceHash, config, clientHash, serverHash, configurations)
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
	return Result{SpecPath: path}, nil
}

func committedSourceIdentity(workspace string) (string, string, error) {
	status := exec.Command("git", "-C", workspace, "diff", "--quiet", "HEAD", "--")
	if err := status.Run(); err != nil {
		return "", "", errors.New("tracked workspace changes must be committed before final preparation")
	}
	command := exec.Command("git", "-C", workspace, "rev-parse", "HEAD")
	raw, err := command.Output()
	commit := strings.TrimSpace(string(raw))
	if err != nil || !hexDigest(commit, 20) {
		return "", "", errors.Join(err, errors.New("repository commit identity is unavailable"))
	}
	archive := exec.Command("git", "-C", workspace, "archive", "--format=tar", "HEAD")
	pipe, err := archive.StdoutPipe()
	if err != nil {
		return "", "", err
	}
	if err := archive.Start(); err != nil {
		return "", "", err
	}
	hash, _, hashErr := hashReader(pipe)
	waitErr := archive.Wait()
	return commit, hash, errors.Join(hashErr, waitErr)
}

func hexDigest(value string, bytes int) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == bytes
}
