package node

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

func (observer *nodeObserver) compose(ctx context.Context, arguments ...string) ([]byte, error) {
	args := append([]string{"compose", "-p", observer.project, "-f", observer.composeFile}, arguments...)
	return observer.docker(ctx, args...)
}

func (observer *nodeObserver) docker(ctx context.Context, arguments ...string) ([]byte, error) {
	return observer.dockerBounded(ctx, 2<<20, 2<<20, arguments...)
}

func (observer *nodeObserver) dockerBounded(ctx context.Context, stdoutLimit, stderrLimit int, arguments ...string) ([]byte, error) {
	commandContext, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(commandContext, "docker", arguments...)
	command.Env = append(os.Environ(), "ARDENTS_NODE_ROOT="+observer.input.FixtureRoot,
		"ARDENTS_NODE_IMAGE_TAG="+observer.imageTag, "ARDENTS_NODE_SOURCE_DIGEST="+observer.sourceDigest,
		"ARDENTS_NODE_BUILD_CONTEXT="+observer.sourceRoot)
	stdout, stderr := byteio.NewBuffer(stdoutLimit), byteio.NewBuffer(stderrLimit)
	command.Stdout, command.Stderr = stdout, stderr
	runErr := command.Run()
	if stdout.Overflowed() || stderr.Overflowed() {
		return stdout.Bytes(), invalidNodeCampaign(fmt.Errorf("docker %v output exceeded its evidence bound", arguments))
	}
	if runErr != nil {
		return stdout.Bytes(), invalidNodeCampaign(fmt.Errorf("docker %v: %w: %s", arguments, runErr, stderr.Bytes()))
	}
	return stdout.Bytes(), nil
}

func (observer *nodeObserver) serviceID(ctx context.Context, service string) (string, error) {
	raw, err := observer.compose(ctx, "ps", "-q", service)
	if err != nil {
		return "", err
	}
	id := string(bytesTrimSpace(raw))
	if len(id) < 12 || len(id) > 64 {
		return "", invalidNodeCampaign(errors.New("node service container identity is invalid"))
	}
	return id, nil
}

func bytesTrimSpace(raw []byte) []byte {
	start, end := 0, len(raw)
	for start < end && (raw[start] == ' ' || raw[start] == '\r' || raw[start] == '\n' || raw[start] == '\t') {
		start++
	}
	for end > start && (raw[end-1] == ' ' || raw[end-1] == '\r' || raw[end-1] == '\n' || raw[end-1] == '\t') {
		end--
	}
	return raw[start:end]
}
