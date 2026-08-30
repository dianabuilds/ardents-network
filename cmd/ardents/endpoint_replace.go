package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/endpoint/portable"
	"github.com/dianabuilds/ardents-network/internal/endpoint/replacement"
	"github.com/dianabuilds/ardents-network/internal/release"
)

const portableUnitName = "ardents-endpoint.service"

// runEndpointReplace owns the explicit participant-controlled Endpoint
// replacement action. The bundle is a local path supplied for this invocation;
// this command does not fetch, poll, schedule, or accept an arbitrary
// service-manager target.
func runEndpointReplace(ctx context.Context, bundleRoot string, output io.Writer) error {
	if runtime.GOOS != "linux" {
		return errors.New("endpoint replacement is available only on Linux")
	}
	if bundleRoot == "" {
		return errors.New("endpoint replacement bundle is required")
	}
	config, err := portable.DefaultConfig()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	replacementRoot := filepath.Join(config.StateHome, "replacement")
	running, err := replacement.VerifyRunning(replacementRoot, executable)
	if err != nil {
		return err
	}
	if running.State != replacement.StateCurrent {
		return errors.New("endpoint replacement requires the current enrolled program")
	}
	inputs, authorization, err := openReplacementAuthorization(ctx, config.StateHome, bundleRoot, "replacement")
	if err != nil {
		return err
	}
	result, replaceErr := replacement.Replace(ctx, replacement.Operation{Request: replacement.Request{StateRoot: replacementRoot,
		Artifact: inputs.Artifact, Authorization: authorization}, ProgramPath: executable, Unit: endpointUserUnit{},
		SelfTest: endpointReplacementSelfTest{program: executable}})
	recoveryProgram := ""
	if result.State == "rollback-authorization-required" {
		recoveryProgram, _ = replacement.RollbackProgramPath(replacementRoot)
	}
	if writeErr := writeReplacementResult(output, "endpoint-replacement", result, recoveryProgram); writeErr != nil {
		return errors.Join(replaceErr, writeErr)
	}
	return replaceErr
}

// runEndpointRollback is deliberately executable only by the retained exact
// predecessor after a failed candidate self-test. The normal program path may
// contain the broken candidate; this command therefore verifies its own
// recovery-copy path before it opens a fresh local Release bundle.
func runEndpointRollback(ctx context.Context, bundleRoot string, output io.Writer) error {
	if runtime.GOOS != "linux" {
		return errors.New("endpoint replacement is available only on Linux")
	}
	if bundleRoot == "" {
		return errors.New("endpoint rollback bundle is required")
	}
	config, err := portable.DefaultConfig()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	replacementRoot := filepath.Join(config.StateHome, "replacement")
	targetProgram, err := replacement.VerifyRollbackProgram(replacementRoot, executable)
	if err != nil {
		return err
	}
	inputs, authorization, err := openReplacementAuthorization(ctx, config.StateHome, bundleRoot, "rollback")
	if err != nil {
		return err
	}
	result, rollbackErr := replacement.Rollback(ctx, replacement.Operation{Request: replacement.Request{StateRoot: replacementRoot,
		Artifact: inputs.Artifact, Authorization: authorization}, ProgramPath: targetProgram, Unit: endpointUserUnit{},
		SelfTest: endpointReplacementSelfTest{program: targetProgram}})
	if writeErr := writeReplacementResult(output, "endpoint-rollback", result, ""); writeErr != nil {
		return errors.Join(rollbackErr, writeErr)
	}
	return rollbackErr
}

// runEndpointReplacementRecovery exposes only the durable Endpoint-replacement
// recovery classification. It cannot start, replace, or roll back a program.
// When a failed self-test has a retained predecessor, it reports that exact
// bounded recovery path so the Owner can supply a fresh Release-authorized rollback
// bundle to that predecessor program.
func runEndpointReplacementRecovery(output io.Writer) error {
	if runtime.GOOS != "linux" {
		return errors.New("endpoint replacement is available only on Linux")
	}
	config, err := portable.DefaultConfig()
	if err != nil {
		return err
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	replacementRoot := filepath.Join(config.StateHome, "replacement")
	result, recoverErr := replacement.Recover(replacementRoot, executable)
	recoveryProgram := ""
	if result.State == "rollback-authorization-required" {
		recoveryProgram, _ = replacement.RollbackProgramPath(replacementRoot)
	}
	if writeErr := writeReplacementResult(output, "endpoint-recovery", result, recoveryProgram); writeErr != nil {
		return errors.Join(recoverErr, writeErr)
	}
	return recoverErr
}

func openReplacementAuthorization(ctx context.Context, stateHome, bundleRoot, operation string) (release.Inputs, release.Authorization, error) {
	inputs, err := replacement.LoadBundle(bundleRoot, time.Now().UTC())
	if err != nil {
		return release.Inputs{}, release.Authorization{}, err
	}
	verifier, err := release.Open(filepath.Join(stateHome, "floors", "release-decision"))
	if err != nil {
		return release.Inputs{}, release.Authorization{}, err
	}
	decision := verifier.Evaluate(ctx, inputs)
	closeErr := verifier.Close()
	if closeErr != nil {
		return release.Inputs{}, release.Authorization{}, closeErr
	}
	if decision.Outcome != release.OutcomeReleaseAccepted {
		return release.Inputs{}, release.Authorization{}, fmt.Errorf("endpoint %s Release decision is not accepted: %s (%s)", operation, decision.Outcome, decision.Notice)
	}
	authorization, ok := decision.Authorization()
	if !ok {
		return release.Inputs{}, release.Authorization{}, fmt.Errorf("endpoint %s accepted Release decision lacks authorization", operation)
	}
	return inputs, authorization, nil
}

type endpointUserUnit struct{}

func (endpointUserUnit) Stop(ctx context.Context) error  { return runUserSystemctl(ctx, "stop") }
func (endpointUserUnit) Start(ctx context.Context) error { return runUserSystemctl(ctx, "start") }

func runUserSystemctl(ctx context.Context, operation string) error {
	if operation != "stop" && operation != "start" {
		return errors.New("endpoint replacement systemd operation is invalid")
	}
	command := exec.CommandContext(ctx, "systemctl", "--user", operation, portableUnitName)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Run(); err != nil {
		return errors.New("endpoint replacement could not control its user unit")
	}
	return nil
}

type endpointReplacementSelfTest struct{ program string }

func (test endpointReplacementSelfTest) Check(ctx context.Context, stateRoot string) error {
	command := exec.CommandContext(ctx, test.program, "endpoint", "replacement-self-test", stateRoot)
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Run(); err != nil {
		return errors.New("endpoint replacement candidate self-test failed")
	}
	return nil
}

func writeReplacementResult(output io.Writer, kind string, result replacement.Result, recoveryProgram string) error {
	encoder := json.NewEncoder(output)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(struct {
		Kind, State     string
		Release         string `json:"release"`
		Version         int64  `json:"release_version"`
		RecoveryProgram string `json:"recovery_program,omitempty"`
	}{Kind: kind, State: result.State, Release: result.Current.ReleaseID,
		Version: result.Current.ReleaseVersion, RecoveryProgram: recoveryProgram})
}
