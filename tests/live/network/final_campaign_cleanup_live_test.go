//go:build live

package network_test

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func newFinalProjectToken() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func prepareFinalWorkerRoot(token string) (string, error) {
	secret := os.Getenv("ARDENTS_BLOCKED_SECRET_ROOT")
	if !filepath.IsAbs(secret) || token == "" {
		return "", errors.New("final worker root inputs are invalid")
	}
	parent := filepath.Join(secret, "measurements", "workers")
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
	secret := os.Getenv("ARDENTS_BLOCKED_SECRET_ROOT")
	parent := filepath.Clean(filepath.Join(secret, "measurements", "workers"))
	clean := filepath.Clean(root)
	if !filepath.IsAbs(secret) || filepath.Dir(clean) != parent || len(filepath.Base(clean)) != 24 {
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
