//go:build live

package network_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const maximumFinalWorkerStream = 16 << 20

type finalWorkerCapture struct {
	buffer   bytes.Buffer
	written  int64
	overflow bool
	limit    int
}

func (capture *finalWorkerCapture) Write(value []byte) (int, error) {
	capture.written += int64(len(value))
	limit := capture.limit
	if limit == 0 {
		limit = maximumFinalWorkerStream
	}
	remaining := limit - capture.buffer.Len()
	if remaining > 0 {
		kept := len(value)
		if kept > remaining {
			kept = remaining
		}
		_, _ = capture.buffer.Write(value[:kept])
	}
	if capture.written > int64(limit) {
		capture.overflow = true
	}
	return len(value), nil
}

func runFinalCellWorker(schedule finalRunnerSchedule, cell, test string) ([]finalWorkerResult, error) {
	projectToken, err := newFinalProjectToken()
	if err != nil {
		return nil, err
	}
	if err := verifyFinalRunnerSupply(schedule, projectToken); err != nil {
		return nil, errors.Join(err, cleanupFinalWorkerProjects(projectToken))
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, errors.Join(err, cleanupFinalWorkerProjects(projectToken))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, executable, "-test.run", "^"+test+"$", "-test.count=1", "-test.v")
	command.Env = finalWorkerEnvironment(map[string]string{
		"ARDENTS_BLOCKED_CELL_WORKER":  "1",
		"ARDENTS_FINAL_CELL":           cell,
		"ARDENTS_WEBTUNNEL_CLIENT":     os.Getenv("ARDENTS_BLOCKED_CLIENT"),
		"ARDENTS_WEBTUNNEL_SERVER":     os.Getenv("ARDENTS_BLOCKED_SERVER"),
		"ARDENTS_LIVE_TOOL_IMAGE":      schedule.ToolImageID,
		"ARDENTS_FINAL_PRODUCT_IMAGE":  schedule.ProductImageID,
		"ARDENTS_BLOCKED_COMPOSE_FILE": os.Getenv("ARDENTS_BLOCKED_COMPOSE_FILE"),
		"ARDENTS_FINAL_PROJECT_TOKEN":  projectToken,
	})
	stdout, stderr, err := runFinalBoundedProcess(command, maximumFinalWorkerStream)
	composeErr := verifyFinalRuntimeCompose(schedule)
	cleanupErr := cleanupFinalWorkerProjects(projectToken)
	if ctx.Err() != nil {
		return nil, errors.New("final worker execution exceeded its bound")
	}
	if composeErr != nil || cleanupErr != nil {
		return nil, errors.Join(composeErr, cleanupErr)
	}
	if err != nil {
		return nil, fmt.Errorf("worker %s failed: %w: %s", test, err, bytes.TrimSpace(stderr))
	}
	return decodeFinalWorkerResults(stdout)
}

func runFinalBoundedProcess(command *exec.Cmd, limit int) ([]byte, []byte, error) {
	prepareFinalProcess(command)
	command.WaitDelay = 5 * time.Second
	command.Cancel = func() error { return terminateFinalProcess(command) }
	stdout := finalWorkerCapture{limit: limit}
	stderr := finalWorkerCapture{limit: limit}
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if stdout.overflow || stderr.overflow {
		return nil, nil, errors.New("final process output exceeded its bound")
	}
	return stdout.buffer.Bytes(), stderr.buffer.Bytes(), err
}

func finalWorkerEnvironment(values map[string]string) []string {
	result := make([]string, 0, len(os.Environ())+len(values))
	for _, current := range os.Environ() {
		name, _, _ := strings.Cut(current, "=")
		if _, replaced := values[name]; !replaced {
			result = append(result, current)
		}
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		value := values[name]
		result = append(result, name+"="+value)
	}
	return result
}

func decodeFinalWorkerResults(output []byte) ([]finalWorkerResult, error) {
	var result []finalWorkerResult
	scanner := bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		var value finalWorkerResult
		if json.Unmarshal(line, &value) == nil && value.Schema == "ardents-h3-final-worker-cell-v1" {
			result = append(result, value)
		}
	}
	if err := scanner.Err(); err != nil || len(result) == 0 {
		return nil, errors.Join(err, errors.New("worker emitted no final cell result"))
	}
	return result, nil
}
