package servicesmoke

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type dockerObserver struct {
	input                                                  Config
	project, image, sourceCommit, generation, evidenceFile string
	runtimeUser                                            string
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
		"ARDENTS_SERVICE_RUNTIME_USER="+observer.runtimeUser)
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
		return "", errors.New("stage 3 campaign requires a clean committed source tree")
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
