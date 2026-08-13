package routesmoke

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (observer dockerObserver) compose(ctx context.Context, timeout time.Duration, arguments ...string) ([]byte, error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := append([]string{"compose", "-p", observer.project, "-f", observer.input.ComposeFile}, arguments...)
	command := exec.CommandContext(commandCtx, "docker", args...)
	command.Env = append(os.Environ(), "ARDENTS_ROUTE_ROOT="+observer.input.FixtureRoot,
		"ARDENTS_ROUTE_EVIDENCE="+observer.input.EvidenceRoot, "ARDENTS_ROUTE_IMAGE_TAG="+observer.image,
		"ARDENTS_ROUTE_SOURCE_DIGEST="+observer.sourceDigest, "ARDENTS_ROUTE_BUILD_CONTEXT="+observer.input.SourceRoot,
		"ARDENTS_ROUTE_RUNTIME_USER="+runtimeUser())
	stdout, stderr := byteio.NewBuffer(2<<20), byteio.NewBuffer(2<<20)
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("docker compose %s: %w: %s", strings.Join(arguments, " "), err, stderr.Bytes())
	}
	if stdout.Overflowed() || stderr.Overflowed() {
		return nil, errors.New("docker output exceeded 2 MiB")
	}
	return stdout.Bytes(), nil
}

func (observer dockerObserver) docker(ctx context.Context, timeout time.Duration, arguments ...string) ([]byte, error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(commandCtx, "docker", arguments...)
	stdout, stderr := byteio.NewBuffer(2<<20), byteio.NewBuffer(2<<20)
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("docker %s: %w: %s", strings.Join(arguments, " "), err, stderr.Bytes())
	}
	if stdout.Overflowed() || stderr.Overflowed() {
		return nil, errors.New("docker output exceeded 2 MiB")
	}
	return stdout.Bytes(), nil
}

func (observer dockerObserver) serviceID(ctx context.Context, service string) (string, error) {
	raw, err := observer.compose(ctx, time.Minute, "ps", "-q", service)
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if !validContainerID(value) {
		return "", errors.New("route role container identity is invalid")
	}
	return value, nil
}

func validContainerID(value string) bool {
	if len(value) < 12 || len(value) > 64 {
		return false
	}
	for _, char := range value {
		if char < '0' || char > '9' && char < 'a' || char > 'f' {
			return false
		}
	}
	return true
}

func (observer dockerObserver) waitContainer(ctx context.Context, containerID string) error {
	raw, err := observer.docker(ctx, time.Minute, "wait", containerID)
	if err != nil {
		return err
	}
	exitCode := strings.TrimSpace(string(raw))
	if exitCode != "0" {
		return fmt.Errorf("container %.12s exited with status %s before complete evidence", containerID, exitCode)
	}
	return nil
}

func (observer dockerObserver) down(ctx context.Context) {
	_, _ = observer.compose(ctx, time.Minute, "down", "--remove-orphans", "--volumes")
}

func (observer dockerObserver) cleanup(ctx context.Context) error {
	if _, err := observer.compose(ctx, time.Minute, "down", "--remove-orphans", "--volumes"); err != nil {
		return fmt.Errorf("route smoke Docker cleanup: %w", err)
	}
	containers, err := observer.docker(ctx, time.Minute, "ps", "-a", "--filter",
		"label=com.docker.compose.project="+observer.project, "-q")
	if err != nil {
		return err
	}
	networks, err := observer.docker(ctx, time.Minute, "network", "ls", "--filter",
		"label=com.docker.compose.project="+observer.project, "-q")
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(containers)) != 0 || len(bytes.TrimSpace(networks)) != 0 {
		return errors.New("route smoke Docker cleanup left owned resources")
	}
	return nil
}

func (observer dockerObserver) imageReceipt(ctx context.Context) (string, error) {
	raw, err := observer.docker(ctx, time.Minute, "image", "inspect", "--format",
		`{{.Id}} {{index .Config.Labels "org.opencontainers.image.revision"}}`, observer.image)
	if err != nil {
		return "", fmt.Errorf("inspect Route smoke image: %w", err)
	}
	fields := strings.Fields(string(raw))
	if len(fields) != 2 || !strings.HasPrefix(fields[0], "sha256:") || fields[1] != observer.sourceDigest {
		return "", errors.New("route smoke image identity or source label is invalid")
	}
	return fields[0], nil
}

func cleanSourceDigest(ctx context.Context, root string) (string, error) {
	status := exec.CommandContext(ctx, "git", "status", "--porcelain")
	status.Dir = root
	raw, err := status.Output()
	if err != nil {
		return "", err
	}
	if len(bytes.TrimSpace(raw)) != 0 {
		return "", errors.New("route smoke requires a clean committed source tree")
	}
	command := exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	command.Dir = root
	raw, err = command.Output()
	if err != nil {
		return "", err
	}
	value := strings.TrimSpace(string(raw))
	if len(value) != 40 {
		return "", errors.New("source commit identity is invalid")
	}
	return value, nil
}
