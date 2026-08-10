package siteexperiment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type referenceProcess struct {
	repositoryRoot string
	project        string
	environment    []string
}

func startReferenceApplication(ctx context.Context, repositoryRoot, runDirectory, image, nonce string, sequence int) (*referenceProcess, string, error) {
	directories := make(map[string]string)
	for _, role := range []string{"client", "service", "admin", "authority"} {
		directory := filepath.Join(runDirectory, fmt.Sprintf("attempt-%03d", sequence), role)
		if err := os.MkdirAll(directory, 0o700); err != nil {
			return nil, "", err
		}
		directories[role] = directory
	}
	digest := sha256.Sum256([]byte(fmt.Sprintf("%s:%d", runDirectory, sequence)))
	process := &referenceProcess{
		repositoryRoot: repositoryRoot,
		project:        "ardents-gatec-" + hex.EncodeToString(digest[:8]),
		environment: append(os.Environ(),
			"GATEC_REFERENCE_IMAGE="+image, "GATEC_NONCE="+nonce,
			"GATEC_CLIENT_DIR="+directories["client"], "GATEC_SERVICE_DIR="+directories["service"],
			"GATEC_ADMIN_DIR="+directories["admin"], "GATEC_AUTHORITY_DIR="+directories["authority"],
		),
	}
	if _, err := process.compose(ctx, "up", "--detach", "--no-build", "--pull", "never", "http-application"); err != nil {
		return nil, "", err
	}
	socket := filepath.Join(directories["service"], "app.sock")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Lstat(socket); err == nil && info.Mode()&os.ModeSocket != 0 {
			return process, socket, nil
		}
		select {
		case <-ctx.Done():
			return process, "", ctx.Err()
		case <-time.After(20 * time.Millisecond):
		}
	}
	return process, "", errors.New("Reference HTTP Application socket was not ready")
}

func (process *referenceProcess) compose(ctx context.Context, arguments ...string) ([]byte, error) {
	base := []string{"compose", "--project-name", process.project, "--file", filepath.Join(process.repositoryRoot, "reference-site", "compose.yaml")}
	command := exec.CommandContext(ctx, "docker", append(base, arguments...)...)
	command.Env = process.environment
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("Reference Site Compose: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func (process *referenceProcess) close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err := process.compose(ctx, "down", "--volumes", "--remove-orphans", "--timeout", "5")
	return err
}

func verifyReferenceImage(ctx context.Context, image, sourceSHA string) error {
	command := exec.CommandContext(ctx, "docker", "run", "--rm", "--pull", "never", "--network", "none", "--entrypoint", "/bin/sh", image, "-c", "cat /usr/share/ardents/gate-c-source.sha256")
	output, err := command.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != sourceSHA {
		return errors.New("Reference Site image is not bound to the current source identity")
	}
	return nil
}
