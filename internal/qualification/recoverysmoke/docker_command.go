package recoverysmoke

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type dockerObserver struct {
	input                                                  config
	project, image, sourceCommit, generation, evidenceFile string
	imageID                                                string
	runtimeUser                                            string
	direction                                              string
	gateOffset                                             uint32
}

func (observer dockerObserver) compose(ctx context.Context, timeout time.Duration, arguments ...string) ([]byte, error) {
	args := append([]string{"compose", "-f", observer.input.ComposeFile, "-p", observer.project}, arguments...)
	return observer.command(ctx, timeout, "docker", args...)
}

func (observer dockerObserver) docker(ctx context.Context, timeout time.Duration, arguments ...string) ([]byte, error) {
	return observer.command(ctx, timeout, "docker", arguments...)
}

func (observer dockerObserver) command(ctx context.Context, timeout time.Duration, name string, arguments ...string) ([]byte, error) {
	bounded, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	command := exec.CommandContext(bounded, name, arguments...)
	command.Dir = observer.input.SourceRoot
	command.Env = append(os.Environ(),
		"ARDENTS_SERVICE_IMAGE_TAG="+observer.image,
		"ARDENTS_SERVICE_SOURCE_COMMIT="+observer.sourceCommit,
		"ARDENTS_SERVICE_BUILD_CONTEXT="+observer.input.SourceRoot,
		"ARDENTS_SERVICE_ROUTE_ROOT="+filepath.Join(observer.input.FixtureRoot, "route"),
		"ARDENTS_SERVICE_GENERATION="+observer.generation,
		"ARDENTS_SERVICE_EVIDENCE_FILE="+observer.evidenceFile,
		"ARDENTS_RECOVERY_EVIDENCE_FILE="+observer.evidenceFile,
		"ARDENTS_RECOVERY_GATE_HOST="+filepath.Join(observer.input.FixtureRoot, "gate"),
		"ARDENTS_SERVICE_RUNTIME_USER="+observer.runtimeUser)
	bytes := observer.input.Bytes
	if bytes == 0 {
		bytes = 64 << 10
	}
	clientSend, clientReceive := bytes, bytes
	publisherSend, publisherReceive := bytes, bytes
	if observer.direction == "client-to-publisher" {
		clientReceive, publisherSend = 0, 0
	}
	if observer.direction == "publisher-to-client" {
		clientSend, publisherReceive = 0, 0
	}
	command.Env = append(command.Env,
		fmt.Sprintf("ARDENTS_CLIENT_SEND_BYTES=%d", clientSend),
		fmt.Sprintf("ARDENTS_CLIENT_RECEIVE_BYTES=%d", clientReceive),
		fmt.Sprintf("ARDENTS_PUBLISHER_SEND_BYTES=%d", publisherSend),
		fmt.Sprintf("ARDENTS_PUBLISHER_RECEIVE_BYTES=%d", publisherReceive),
		"ARDENTS_STREAM_CHUNK_DELAY="+observer.input.ChunkDelay,
		"ARDENTS_STREAM_PROGRESS=1",
		fmt.Sprintf("ARDENTS_STREAM_GATE_OFFSET=%d", observer.gateOffset))
	if observer.gateOffset > 0 {
		command.Env = append(command.Env, "ARDENTS_STREAM_GATE_ROOT=/run/ardents/gate")
	} else {
		command.Env = append(command.Env, "ARDENTS_STREAM_GATE_ROOT=")
	}
	output, err := command.CombinedOutput()
	if bounded.Err() != nil {
		return output, bounded.Err()
	}
	if err != nil {
		return output, errors.New(strings.TrimSpace(string(output)) + ": " + err.Error())
	}
	return output, nil
}

func cleanCommit(ctx context.Context, root string) (string, error) {
	command := exec.CommandContext(ctx, "git", "status", "--porcelain")
	command.Dir = root
	status, err := command.Output()
	if err != nil || len(status) != 0 {
		return "", errors.New("recovery campaign requires a clean committed source tree")
	}
	command = exec.CommandContext(ctx, "git", "rev-parse", "HEAD")
	command.Dir = root
	raw, err := command.Output()
	if err != nil {
		return "", err
	}
	commit := strings.TrimSpace(string(raw))
	if len(commit) != 40 {
		return "", errors.New("source commit identity is invalid")
	}
	return commit, nil
}
