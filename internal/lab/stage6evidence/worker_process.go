package stage6evidence

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const workerProcessLimit = 2 * time.Minute

func executeWorker(executable string, input workerInput) (workerResult, error) {
	return executeWorkerProcess(executable, []string{"worker", "-root"}, input)
}

func executeWorkerProcess(executable string, arguments []string, input workerInput) (workerResult, error) {
	root, token, err := createWorkerRoot()
	if err != nil {
		return workerResult{}, err
	}
	defer func() { _ = removeWorkerRoot(root, token) }()
	if _, err := writeJSON(root, "input.json", input.Schema, input, false); err != nil {
		return workerResult{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), workerProcessLimit)
	defer cancel()
	command := exec.CommandContext(ctx, executable, append(arguments, root)...)
	command.Dir = root
	command.Env = workerEnvironment()
	var stdout, stderr boundedWorkerOutput
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil || ctx.Err() != nil || stdout.overflow || stderr.overflow ||
		stdout.String() != "complete\n" {
		return workerResult{}, errors.Join(err, ctx.Err(),
			fmt.Errorf("S6E1 worker process failed: %s", strings.TrimSpace(stderr.String())))
	}
	if err := exactWorkerInventory(root, "input.json", "owner", "result.json"); err != nil {
		return workerResult{}, err
	}
	var result workerResult
	if err := readCanonicalWorkerFile(filepath.Join(root, "result.json"), &result); err != nil {
		return workerResult{}, err
	}
	if result.Schema != workerResultSchema || result.Cell != input.Cell || result.Ordinal != input.Ordinal ||
		result.Class == "" || result.WorkerPID <= 0 || result.Trace.Cell != input.Cell ||
		result.Trace.Ordinal != input.Ordinal || result.Trace.Operation != input.Predicate ||
		result.Trace.StartOffset != input.StartOffset {
		return workerResult{}, errors.New("S6E1 worker result is invalid")
	}
	if err := removeWorkerRoot(root, token); err != nil {
		return workerResult{}, err
	}
	root, token = "", ""
	return result, nil
}

func createWorkerRoot() (string, string, error) {
	root, err := os.MkdirTemp("", "ardents-stage6-worker-")
	if err != nil {
		return "", "", err
	}
	token := filepath.Base(root)
	if err := os.WriteFile(filepath.Join(root, "owner"), []byte(token), 0o600); err != nil {
		_ = os.Remove(root)
		return "", "", err
	}
	return root, token, nil
}

func removeWorkerRoot(root, token string) error {
	if root == "" {
		return nil
	}
	absolute, err := filepath.Abs(root)
	temporary, tempErr := filepath.Abs(os.TempDir())
	relative, relErr := filepath.Rel(temporary, absolute)
	owner, readErr := os.ReadFile(filepath.Join(absolute, "owner"))
	if err != nil || tempErr != nil || relErr != nil || relative == "." || strings.HasPrefix(relative, "..") ||
		filepath.Base(absolute) != token || !bytes.Equal(owner, []byte(token)) || readErr != nil {
		return errors.New("refused to remove an unowned S6E1 worker root")
	}
	return os.RemoveAll(absolute)
}

func workerEnvironment() []string {
	allowed := map[string]bool{"PATH": true, "SystemRoot": true, "WINDIR": true, "TEMP": true, "TMP": true}
	result := []string{}
	for _, value := range os.Environ() {
		name, _, _ := strings.Cut(value, "=")
		if allowed[name] {
			result = append(result, value)
		}
	}
	return result
}

type boundedWorkerOutput struct {
	bytes.Buffer
	overflow bool
}

func (output *boundedWorkerOutput) Write(value []byte) (int, error) {
	const maximum = 64 << 10
	original := len(value)
	if output.Len()+len(value) > maximum {
		value, output.overflow = value[:max(0, maximum-output.Len())], true
	}
	_, _ = output.Buffer.Write(value)
	return original, nil
}
