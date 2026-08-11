package siteexperiment

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

const maximumRoleErrorBytes = 8 * 1024

type referenceRoleState struct {
	ID     string `json:"Id"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Running  bool `json:"Running"`
		ExitCode int  `json:"ExitCode"`
	} `json:"State"`
}

func (process *referenceProcess) captureContainerIDs(ctx context.Context) error {
	ids, err := process.compose(ctx, "ps", "--all", "--quiet")
	if err != nil {
		return err
	}
	fields := strings.Fields(string(ids))
	if len(fields) != len(referenceRoles) {
		return errors.New("reference Site process set is incomplete")
	}
	process.containerIDs = append([]string(nil), fields...)
	return nil
}

func (process *referenceProcess) detectFailedRole(ctx context.Context) error {
	states, err := process.referenceRoleStates(ctx)
	if err != nil {
		return err
	}
	failed := firstFailedReferenceRole(states)
	if failed == nil {
		return nil
	}
	return process.describeFailedRole(ctx, failed)
}

func (process *referenceProcess) referenceRoleStates(ctx context.Context) ([]referenceRoleState, error) {
	if len(process.containerIDs) != len(referenceRoles) {
		return nil, errors.New("reference Site container identity set is incomplete")
	}
	data, err := exec.CommandContext(ctx, "docker", append([]string{"inspect"}, process.containerIDs...)...).Output()
	if err != nil {
		return nil, fmt.Errorf("inspect reference Site roles: %w", err)
	}
	return parseReferenceRoleStates(data, process.containerIDs)
}

func (process *referenceProcess) describeFailedRole(ctx context.Context, failed *referenceRoleState) error {
	role := failed.Config.Labels["com.docker.compose.service"]
	command := exec.CommandContext(ctx, "docker", "logs", "--tail", "20", failed.ID)
	reader, writer := io.Pipe()
	command.Stdout, command.Stderr = writer, writer
	type collectedLog struct {
		value string
		err   error
	}
	collected := make(chan collectedLog, 1)
	go func() {
		value, err := collectBoundedRoleLog(reader)
		collected <- collectedLog{value: value, err: err}
	}()
	logErr := command.Run()
	closeErr := writer.Close()
	output := <-collected
	readCloseErr := reader.Close()
	failure := fmt.Errorf("reference Site role %q exited with status %d: %s", role, failed.State.ExitCode, output.value)
	return errors.Join(failure, logErr, closeErr, output.err, readCloseErr)
}

func collectBoundedRoleLog(reader io.Reader) (string, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maximumRoleErrorBytes+1))
	if err != nil {
		return "", err
	}
	_, drainErr := io.Copy(io.Discard, reader)
	truncated := len(data) > maximumRoleErrorBytes
	if truncated {
		data = data[:maximumRoleErrorBytes]
	}
	value := strings.TrimSpace(string(data))
	if truncated {
		value += " [truncated]"
	}
	return value, drainErr
}

func parseReferenceRoleStates(data []byte, expectedIDs []string) ([]referenceRoleState, error) {
	var states []referenceRoleState
	if err := json.Unmarshal(data, &states); err != nil || len(states) != len(expectedIDs) {
		return nil, errors.New("reference Site role state is invalid")
	}
	expected := make(map[string]bool, len(expectedIDs))
	for _, id := range expectedIDs {
		expected[id] = true
	}
	seenRoles := make(map[string]bool, len(referenceRoles))
	for index := range states {
		role := states[index].Config.Labels["com.docker.compose.service"]
		if !expected[states[index].ID] || !isReferenceRole(role) || seenRoles[role] {
			return nil, errors.New("reference Site role state identity is invalid")
		}
		seenRoles[role] = true
	}
	if len(seenRoles) != len(referenceRoles) {
		return nil, errors.New("reference Site role state set is incomplete")
	}
	return states, nil
}

func firstFailedReferenceRole(states []referenceRoleState) *referenceRoleState {
	for index := range states {
		if !states[index].State.Running && states[index].State.ExitCode != 0 {
			return &states[index]
		}
	}
	return nil
}

func isReferenceRole(role string) bool {
	for _, allowed := range referenceRoles {
		if role == allowed {
			return true
		}
	}
	return false
}
