//go:build live

package network_test

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maximumFinalWorkerInput = 256 << 20

func newFinalProjectToken() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func prepareFinalWorkerRoot(token string) (string, error) {
	staging := os.Getenv("ARDENTS_BLOCKED_STAGING_ROOT")
	if !filepath.IsAbs(staging) || token == "" {
		return "", errors.New("final worker root inputs are invalid")
	}
	parent := filepath.Join(staging, "workers")
	if err := os.MkdirAll(parent, 0o700); err != nil {
		return "", err
	}
	root := filepath.Join(parent, token)
	if err := os.Mkdir(root, 0o700); err != nil {
		return "", err
	}
	return root, nil
}

func cleanupFinalWorkerRoot(root string) error {
	staging := os.Getenv("ARDENTS_BLOCKED_STAGING_ROOT")
	parent := filepath.Clean(filepath.Join(staging, "workers"))
	clean := filepath.Clean(root)
	if !filepath.IsAbs(staging) || filepath.Dir(clean) != parent || len(filepath.Base(clean)) != 24 {
		return errors.New("refused to clean an unowned final worker root")
	}
	if _, err := hex.DecodeString(filepath.Base(clean)); err != nil {
		return errors.New("refused to clean a malformed final worker root")
	}
	if err := os.RemoveAll(clean); err != nil {
		return err
	}
	if _, err := os.Lstat(clean); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(err, errors.New("final worker root remained after cleanup"))
	}
	if err := os.Remove(parent); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func prepareFinalWorkerInputs(root, client, server, compose, clientHash, serverHash, composeHash string) (
	string, string, string, error,
) {
	directory := filepath.Join(root, "input")
	if err := os.Mkdir(directory, 0o700); err != nil {
		return "", "", "", err
	}
	paths := []string{filepath.Join(directory, "client"), filepath.Join(directory, "server"),
		filepath.Join(directory, "compose.yml")}
	for index, source := range []string{client, server, compose} {
		expected := []string{clientHash, serverHash, composeHash}[index]
		if err := copyFinalWorkerInput(source, paths[index], expected); err != nil {
			return "", "", "", err
		}
	}
	return paths[0], paths[1], paths[2], nil
}

func copyFinalWorkerInput(source, target, expectedHash string) error {
	info, err := os.Lstat(source)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() ||
		info.Size() < 1 || info.Size() > maximumFinalWorkerInput || len(expectedHash) != 64 {
		return errors.Join(err, errors.New("final worker input is not a bounded regular file"))
	}
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	opened, err := input.Stat()
	if err != nil || !os.SameFile(info, opened) || info.Size() != opened.Size() ||
		info.ModTime() != opened.ModTime() {
		_ = input.Close()
		return errors.Join(err, errors.New("final worker input changed before copy"))
	}
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o500)
	if err != nil {
		_ = input.Close()
		return err
	}
	digest := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(output, digest), io.LimitReader(input, maximumFinalWorkerInput+1))
	after, statErr := input.Stat()
	closeErr := errors.Join(input.Close(), output.Sync(), output.Close())
	actualHash := hex.EncodeToString(digest.Sum(nil))
	if copyErr != nil || statErr != nil || closeErr != nil || written != info.Size() ||
		!os.SameFile(info, after) || info.Size() != after.Size() || info.ModTime() != after.ModTime() ||
		actualHash != expectedHash {
		return errors.Join(copyErr, statErr, closeErr, errors.New("final worker input copy is incomplete"))
	}
	return nil
}

func cleanupFinalWorkerInputs(root string) error {
	directory := filepath.Join(root, "input")
	if err := os.RemoveAll(directory); err != nil {
		return err
	}
	if _, err := os.Lstat(directory); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(err, errors.New("final worker input copies remained after cleanup"))
	}
	return nil
}

func finalProjectName(development string) string {
	token := os.Getenv("ARDENTS_FINAL_PROJECT_TOKEN")
	if token == "" {
		return development
	}
	return "ardents-final-" + token
}

func cleanupFinalWorkerProjects(token string) error {
	project := "ardents-final-" + token
	if err := errors.Join(removeFinalDockerKind(project, "container", "ps", "-aq"),
		removeFinalDockerKind(project, "network", "network", "ls", "-q"),
		removeFinalDockerKind(project, "volume", "volume", "ls", "-q")); err != nil {
		return err
	}
	return verifyFinalWorkerProjectsGone(project)
}

func verifyFinalWorkerProjectsGone(project string) error {
	for _, arguments := range [][]string{
		{"ps", "-aq", "--filter", "label=com.docker.compose.project=" + project},
		{"network", "ls", "-q", "--filter", "label=com.docker.compose.project=" + project},
		{"volume", "ls", "-q", "--filter", "label=com.docker.compose.project=" + project},
	} {
		output, err := finalSupplyOutput("docker", arguments...)
		if err != nil {
			return err
		}
		if len(strings.Fields(string(output))) != 0 {
			return errors.New("final worker Docker residue remained after cleanup")
		}
	}
	return nil
}

func removeFinalDockerKind(project, kind string, list ...string) error {
	arguments := append(list, "--filter", "label=com.docker.compose.project="+project)
	output, err := finalSupplyOutput("docker", arguments...)
	if err != nil {
		return err
	}
	identities := strings.Fields(string(output))
	if len(identities) == 0 {
		return nil
	}
	remove := []string{kind, "rm", "--force"}
	_, err = finalSupplyOutput("docker", append(remove, identities...)...)
	return err
}

func verifiedFinalWorkerResiduals() []finalRunnerResidual {
	kinds := []string{"process", "listener", "socket", "namespace", "mount", "file", "queue", "timer",
		"cgroup", "publishable-secret"}
	result := make([]finalRunnerResidual, 0, len(kinds))
	for _, kind := range kinds {
		result = append(result, finalRunnerResidual{Kind: kind, Owner: "none"})
	}
	return result
}
