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

	"github.com/dianabuilds/ardents-network/internal/qualification/byteio"
)

type dockerObserver struct {
	input                                                  config
	project, image, sourceCommit, generation, evidenceFile string
	imageID                                                string
	runtimeUser                                            string
	direction                                              string
	gateOffset                                             uint32
	gateOffsets                                            []uint32
	streamLifetime                                         string
}

func (observer dockerObserver) compose(ctx context.Context, timeout time.Duration, arguments ...string) ([]byte, error) {
	args := append([]string{"compose", "-f", observer.input.ComposeFile, "-p", observer.project}, arguments...)
	return observer.command(ctx, timeout, "docker", args...)
}

func (observer dockerObserver) docker(ctx context.Context, timeout time.Duration, arguments ...string) ([]byte, error) {
	return observer.command(ctx, timeout, "docker", arguments...)
}

func (observer dockerObserver) command(ctx context.Context, timeout time.Duration, name string, arguments ...string) ([]byte, error) {
	command, bounded, cancel := observer.configuredCommand(ctx, timeout, name, arguments...)
	defer cancel()
	output, err := command.CombinedOutput()
	return commandOutput(output, err, bounded.Err())
}

func (observer dockerObserver) commandBounded(ctx context.Context, timeout time.Duration, maximum int,
	name string, arguments ...string) ([]byte, error) {
	command, bounded, cancel := observer.configuredCommand(ctx, timeout, name, arguments...)
	defer cancel()
	output := byteio.NewBuffer(maximum)
	command.Stdout, command.Stderr = output, output
	err := command.Run()
	if bounded.Err() != nil {
		return output.Bytes(), bounded.Err()
	}
	if output.Overflowed() {
		return output.Bytes(), errors.Join(fmt.Errorf("command output exceeded %d bytes", maximum), err)
	}
	return commandOutput(output.Bytes(), err, nil)
}

func (observer dockerObserver) configuredCommand(ctx context.Context, timeout time.Duration, name string,
	arguments ...string) (*exec.Cmd, context.Context, context.CancelFunc) {
	bounded, cancel := context.WithTimeout(ctx, timeout)
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
	command.Env = append(command.Env, observer.streamEnvironment(bytes)...)
	if len(observer.gateOffsets) > 0 {
		encoded := make([]string, len(observer.gateOffsets))
		for index, offset := range observer.gateOffsets {
			encoded[index] = fmt.Sprintf("%d", offset)
		}
		command.Env = append(command.Env, "ARDENTS_STREAM_GATE_OFFSET=", "ARDENTS_STREAM_GATE_OFFSETS="+strings.Join(encoded, ","))
	} else {
		command.Env = append(command.Env, fmt.Sprintf("ARDENTS_STREAM_GATE_OFFSET=%d", observer.gateOffset),
			"ARDENTS_STREAM_GATE_OFFSETS=")
	}
	if observer.gateOffset > 0 || len(observer.gateOffsets) > 0 {
		command.Env = append(command.Env, "ARDENTS_STREAM_GATE_ROOT=/run/ardents/gate")
	} else {
		command.Env = append(command.Env, "ARDENTS_STREAM_GATE_ROOT=")
	}
	return command, bounded, cancel
}

func commandOutput(output []byte, runErr, contextErr error) ([]byte, error) {
	if contextErr != nil {
		return output, contextErr
	}
	if runErr != nil {
		diagnostic := strings.TrimSpace(string(output))
		if diagnostic == "" {
			return output, fmt.Errorf("command failed: %w", runErr)
		}
		return output, fmt.Errorf("%s: %w", diagnostic, runErr)
	}
	return output, nil
}

func (observer dockerObserver) streamEnvironment(bytes uint32) []string {
	clientSend, clientReceive, publisherSend, publisherReceive := streamBounds(bytes, observer.direction)
	return []string{
		fmt.Sprintf("ARDENTS_CLIENT_SEND_BYTES=%d", clientSend),
		fmt.Sprintf("ARDENTS_CLIENT_RECEIVE_BYTES=%d", clientReceive),
		fmt.Sprintf("ARDENTS_PUBLISHER_SEND_BYTES=%d", publisherSend),
		fmt.Sprintf("ARDENTS_PUBLISHER_RECEIVE_BYTES=%d", publisherReceive),
		"ARDENTS_STREAM_CHUNK_DELAY=" + observer.input.ChunkDelay,
		"ARDENTS_STREAM_PROGRESS=1",
		"ARDENTS_STREAM_LIFETIME=" + observer.streamLifetime,
	}
}

func (observer dockerObserver) forRecoveryOperation(direction string) dockerObserver {
	observer.direction, observer.streamLifetime = direction, recoveryOperationLifetime
	return observer
}

func recoveryDownArguments() []string {
	return []string{"--profile", "s42", "down", "-v", "--remove-orphans"}
}

func (observer dockerObserver) resetRecoveryTopology(ctx context.Context, timeout time.Duration) error {
	return runRecoveryDown(func(arguments ...string) ([]byte, error) {
		return observer.compose(ctx, timeout, arguments...)
	})
}

func runRecoveryDown(run func(...string) ([]byte, error)) error {
	if _, err := run(recoveryDownArguments()...); err != nil {
		return fmt.Errorf("reset recovery topology: %w", err)
	}
	return nil
}

func streamBounds(bytes uint32, direction string) (uint32, uint32, uint32, uint32) {
	clientSend, clientReceive, publisherSend, publisherReceive := bytes, bytes, bytes, bytes
	if direction == "client-to-publisher" {
		clientReceive, publisherSend = 0, 0
	}
	if direction == "publisher-to-client" {
		clientSend, publisherReceive = 0, 0
	}
	return clientSend, clientReceive, publisherSend, publisherReceive
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
