//go:build live

package network_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"
)

const maximumFinalWorkerStream = 16 << 20

type finalWorkerCapture struct {
	buffer   bytes.Buffer
	line     bytes.Buffer
	written  int64
	overflow bool
	limit    int
	onLine   func([]byte)
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
	capture.observeLines(value)
	return len(value), nil
}

func (capture *finalWorkerCapture) observeLines(value []byte) {
	if capture.onLine == nil {
		return
	}
	for len(value) > 0 {
		index := bytes.IndexByte(value, '\n')
		if index < 0 {
			_, _ = capture.line.Write(value)
			break
		}
		_, _ = capture.line.Write(value[:index])
		capture.onLine(capture.line.Bytes())
		capture.line.Reset()
		value = value[index+1:]
	}
	if capture.line.Len() > 1<<20 {
		capture.overflow = true
		capture.line.Reset()
	}
}

func runFinalCellWorker(schedule finalRunnerSchedule, cell, test string,
	clockOrigin time.Time,
) (results []finalWorkerResult, err error) {
	projectToken, err := newFinalProjectToken()
	if err != nil {
		return nil, err
	}
	if err := verifyFinalRunnerSupply(schedule, projectToken); err != nil {
		return nil, errors.Join(err, cleanupFinalWorkerProjects(projectToken))
	}
	workerRoot, err := prepareFinalWorkerRoot(projectToken)
	if err != nil {
		return nil, errors.Join(err, cleanupFinalWorkerProjects(projectToken))
	}
	rootOwned := true
	defer func() {
		if rootOwned {
			err = errors.Join(err, cleanupFinalWorkerRoot(workerRoot))
		}
	}()
	executable, err := os.Executable()
	if err != nil {
		return nil, errors.Join(err, cleanupFinalWorkerProjects(projectToken))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	workerClient, workerServer, workerCompose, err := prepareFinalWorkerInputs(workerRoot,
		os.Getenv("ARDENTS_BLOCKED_CLIENT"), os.Getenv("ARDENTS_BLOCKED_SERVER"),
		os.Getenv("ARDENTS_BLOCKED_COMPOSE_FILE"), schedule.ClientSHA256, schedule.ServerSHA256,
		schedule.RuntimeCompose.SHA256)
	if err != nil {
		return nil, err
	}
	campaignAnchor, err := finalCampaignMonotonicAnchor(clockOrigin)
	if err != nil {
		return nil, err
	}
	startedOffset := uint64(time.Since(clockOrigin).Milliseconds())
	command := exec.CommandContext(ctx, executable, "-test.run", "^"+test+"$", "-test.count=1", "-test.v")
	cellSeed, ok := finalRunnerCellSeed(schedule, cell)
	if !ok {
		return nil, errors.New("final worker cell seed is absent from the frozen schedule")
	}
	values := map[string]string{
		"ARDENTS_BLOCKED_CELL_WORKER":                    "1",
		"ARDENTS_FINAL_CELL":                             cell,
		"ARDENTS_FINAL_CELL_SEED":                        cellSeed,
		"ARDENTS_WEBTUNNEL_CLIENT":                       workerClient,
		"ARDENTS_WEBTUNNEL_SERVER":                       workerServer,
		"ARDENTS_LIVE_TOOL_IMAGE":                        schedule.ToolImageID,
		"ARDENTS_FINAL_PRODUCT_IMAGE":                    schedule.ProductImageID,
		"ARDENTS_BLOCKED_COMPOSE_FILE":                   workerCompose,
		"ARDENTS_FINAL_PROJECT_TOKEN":                    projectToken,
		"ARDENTS_FINAL_WORKER_ROOT":                      workerRoot,
		"ARDENTS_FINAL_CAMPAIGN_MONOTONIC_ANCHOR_MILLIS": strconv.FormatInt(campaignAnchor, 10),
	}
	canary, err := finalCandidateCanary(cell, os.Getenv("ARDENTS_BLOCKED_CANARY_FILE"))
	if err != nil {
		return nil, err
	}
	if canary != "" {
		values["ARDENTS_FINAL_CANDIDATE_CANARY"] = canary
	}
	command.Env = finalWorkerEnvironment(values)
	results, receipt, err := completeFinalWorkerProcess(command, maximumFinalWorkerStream, clockOrigin,
		cell, workerRoot, os.Getenv("ARDENTS_BLOCKED_SECRET_ROOT"), func() error {
			return errors.Join(cleanupFinalWorkerInputs(workerRoot), verifyFinalRuntimeCompose(schedule),
				cleanupFinalWorkerProjects(projectToken))
		})
	if ctx.Err() != nil {
		return nil, errors.New("final worker execution exceeded its bound")
	}
	if err != nil {
		return nil, fmt.Errorf("worker %s failed: %w", test, err)
	}
	rootOwned = false
	cleanupOffset := uint64(time.Since(clockOrigin).Milliseconds())
	terminalOffset := uint64(receipt.At.Milliseconds())
	completeFinalWorkerEvidence(results, startedOffset, terminalOffset, cleanupOffset)
	return results, nil
}

func finalRunnerCellSeed(schedule finalRunnerSchedule, cell string) (string, bool) {
	if len(schedule.CellOrder) != len(schedule.Seeds) {
		return "", false
	}
	for index, identity := range schedule.CellOrder {
		if identity == cell {
			return schedule.Seeds[index], true
		}
	}
	return "", false
}

func finalCampaignMonotonicAnchor(clockOrigin time.Time) (int64, error) {
	hostMillis, err := linuxMonotonicMillis()
	if err != nil {
		return 0, err
	}
	return time.Since(clockOrigin).Milliseconds() - hostMillis, nil
}

func completeFinalWorkerProcess(command *exec.Cmd, limit int, clockOrigin time.Time, cell, workerRoot, secret string,
	cleanup func() error,
) ([]finalWorkerResult, finalTerminalReceipt, error) {
	stdout, stderr, receipt, runErr := runFinalBoundedWorkerProcess(command, limit, clockOrigin, cell)
	var cleanupErr error
	if cleanup != nil {
		cleanupErr = cleanup()
	}
	if runErr != nil || cleanupErr != nil {
		return nil, receipt, errors.Join(runErr, cleanupErr,
			fmt.Errorf("final worker stderr: %s", bytes.TrimSpace(stderr)))
	}
	results, err := decodeFinalWorkerResults(stdout)
	if err != nil || len(results) != 1 || results[0].CellID != cell || results[0].Terminal != receipt.Terminal {
		return nil, receipt, errors.Join(err,
			errors.New("final worker terminal receipt does not match its selected result"))
	}
	observerArtifact, telemetryArtifact, err := publishFinalWorkerHandoff(workerRoot, secret, cell)
	if err != nil {
		return nil, receipt, err
	}
	results[0].ObserverEvidence, results[0].TelemetryEvidence = observerArtifact, telemetryArtifact
	if err := releaseFinalWorkerRoot(workerRoot); err != nil {
		return nil, receipt, err
	}
	return results, receipt, nil
}

func runFinalBoundedProcess(command *exec.Cmd, limit int) ([]byte, []byte, error) {
	return runFinalBoundedProcessObserved(command, limit, nil)
}

type finalTerminalReceipt struct {
	At       time.Duration
	CellID   string
	Terminal string
	count    uint32
	results  uint32
	invalid  bool
}

func runFinalBoundedWorkerProcess(command *exec.Cmd, limit int,
	clockOrigin time.Time, expectedCell string,
) ([]byte, []byte, finalTerminalReceipt, error) {
	receipt := finalTerminalReceipt{At: -1}
	observe := func(line []byte) {
		value, recognized, decodeErr := decodeFinalTerminalMarker(line)
		if recognized {
			receipt.count++
			if decodeErr != nil || receipt.count != 1 || receipt.results != 0 || value.CellID != expectedCell {
				receipt.invalid = true
				return
			}
			receipt.At = time.Since(clockOrigin)
			receipt.CellID, receipt.Terminal = value.CellID, value.Terminal
			return
		}
		_, recognized, decodeErr = decodeFinalWorkerResultLine(line)
		if !recognized {
			return
		}
		receipt.results++
		if decodeErr != nil || receipt.count != 1 || receipt.results != 1 {
			receipt.invalid = true
		}
	}
	stdout, stderr, err := runFinalBoundedProcessObserved(command, limit, observe)
	if receipt.count != 1 || receipt.results != 1 || receipt.invalid || receipt.At < 0 {
		err = errors.Join(err, errors.New("final worker terminal receipt is missing, duplicated, or invalid"))
	}
	return stdout, stderr, receipt, err
}

func decodeFinalTerminalMarker(line []byte) (struct {
	Schema   string `json:"schema"`
	CellID   string `json:"cell_id"`
	Terminal string `json:"terminal"`
}, bool, error) {
	var probe struct {
		Schema string `json:"schema"`
	}
	var value struct {
		Schema   string `json:"schema"`
		CellID   string `json:"cell_id"`
		Terminal string `json:"terminal"`
	}
	if err := json.Unmarshal(line, &probe); err != nil {
		if bytes.Contains(line, []byte("ardents-h3-final-worker-terminal-v1")) {
			return value, true, err
		}
		return value, false, nil
	}
	if probe.Schema != "ardents-h3-final-worker-terminal-v1" {
		return value, false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF ||
		value.CellID == "" || value.Terminal == "" {
		return value, true, errors.New("final worker terminal marker is malformed")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(line, canonical) {
		return value, true, errors.New("final worker terminal marker is not canonical")
	}
	return value, true, nil
}

func runFinalBoundedProcessObserved(command *exec.Cmd, limit int,
	observe func([]byte),
) ([]byte, []byte, error) {
	prepareFinalProcess(command)
	command.WaitDelay = 5 * time.Second
	command.Cancel = func() error { return terminateFinalProcess(command) }
	stdout := finalWorkerCapture{limit: limit, onLine: observe}
	stderr := finalWorkerCapture{limit: limit}
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	groupErr := terminateFinalProcess(command)
	if groupErr == nil {
		err = errors.Join(err, errors.New("final process retained a descendant after parent exit"))
	} else if !errors.Is(groupErr, os.ErrProcessDone) {
		err = errors.Join(err, fmt.Errorf("verify final process-group cleanup: %w", groupErr))
	}
	if stdout.overflow || stderr.overflow {
		return nil, nil, errors.New("final process output exceeded its bound")
	}
	return stdout.buffer.Bytes(), stderr.buffer.Bytes(), err
}

func finalWorkerEnvironment(values map[string]string) []string {
	result := make([]string, 0, len(os.Environ())+len(values))
	allowed := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "COMSPEC": true,
		"PATHEXT": true, "TEMP": true, "TMP": true, "TMPDIR": true, "DOCKER_HOST": true}
	for _, current := range os.Environ() {
		name, _, _ := strings.Cut(current, "=")
		if !allowed[strings.ToUpper(name)] {
			continue
		}
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
		line := scanner.Bytes()
		value, recognized, err := decodeFinalWorkerResultLine(line)
		if err != nil {
			return nil, err
		}
		if recognized {
			result = append(result, value)
		}
	}
	if err := scanner.Err(); err != nil || len(result) != 1 {
		return nil, errors.Join(err, errors.New("worker must emit exactly one strict final cell result"))
	}
	return result, nil
}

func decodeFinalWorkerResultLine(line []byte) (finalWorkerResult, bool, error) {
	var probe struct {
		Schema string `json:"schema"`
	}
	var value finalWorkerResult
	if err := json.Unmarshal(line, &probe); err != nil {
		if bytes.Contains(line, []byte("ardents-h3-final-worker-cell-v1")) {
			return value, true, err
		}
		return value, false, nil
	}
	if probe.Schema != "ardents-h3-final-worker-cell-v1" {
		return value, false, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(line))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return value, true, errors.New("final worker cell result is malformed")
	}
	canonical, err := json.Marshal(value)
	if err != nil || !bytes.Equal(line, canonical) {
		return value, true, errors.New("final worker cell result is not canonical")
	}
	return value, true, nil
}
