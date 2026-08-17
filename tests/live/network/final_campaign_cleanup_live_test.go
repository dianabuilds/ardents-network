//go:build live

package network_test

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"strings"
)

func newFinalProjectToken() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
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
	return errors.Join(removeFinalDockerKind(project, "container", "ps", "-aq"),
		removeFinalDockerKind(project, "network", "network", "ls", "-q"),
		removeFinalDockerKind(project, "volume", "volume", "ls", "-q"))
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
