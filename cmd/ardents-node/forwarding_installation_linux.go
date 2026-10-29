package main

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/forwarding"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// The distinct Forwarding route never enters Source Installation or the legacy
// Node dispatcher. Initial installation never starts a runtime; preflight grants
// no accepting installed/runtime operation.
func runForwardingInstallation(parent context.Context, args []string, output io.Writer) (returned error) {
	if parent == nil || output == nil || len(args) == 0 {
		return forwarding.ErrInput
	}
	if args[0] == "supervise-transition" {
		return superviseForwardingTransition(parent, args[1:], output)
	}
	coordinator, managed, err := forwarding.OpenTransitionCoordinator(args[0])
	if err != nil {
		return err
	}
	if managed {
		parent = coordinator.Context()
		defer func() { returned = errors.Join(returned, coordinator.Close()) }()
	}
	if parent != nil && output != nil && len(args) == 2 && args[0] == "start-installed" {
		return runInstalledForwarding(parent, args[1], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "inspect-initial-history" {
		return inspectInitialForwarding(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 3 && args[0] == "inspect-start-history" {
		return inspectForwardingStart(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 3 && args[0] == "stop" {
		return stopForwarding(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "start-initial" {
		return startInitialForwarding(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "restart" {
		return startForwardingGeneration(parent, args[1:], output, forwardingRestart)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "start-selected" {
		return startForwardingGeneration(parent, args[1:], output, forwardingSelectedStart)
	}
	if parent != nil && output != nil && len(args) == 7 && args[0] == "install-initial-stopped" {
		return installInitialForwarding(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 7 && args[0] == "replace" {
		return replaceForwarding(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 7 && args[0] == "recover" {
		return recoverForwarding(parent, args[1:], output)
	}
	if parent == nil || output == nil || len(args) != 2 || args[0] != "inspect-inputs" {
		return errors.New("usage: ardents-node forwarding inspect-inputs REQUEST | install-initial-stopped DISTRIBUTION PIN HISTORY ENVIRONMENT NETWORK REQUEST | inspect-initial-history DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING | start-initial DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING | restart DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING | replace DISTRIBUTION HISTORY ENVIRONMENT NETWORK REQUEST BINDING | start-selected DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING | recover DISTRIBUTION HISTORY ENVIRONMENT NETWORK REQUEST JOURNAL | start-installed BINDING | inspect-start-history HISTORY BINDING | stop HISTORY BINDING")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	input, err := forwarding.ReadInputs(ctx, args[1])
	if err == nil {
		err = forwarding.InspectInitialInputs(input)
	}
	if err = errors.Join(err, input.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-inputs-preflight-completed\",\"installed\":false}\n")
	return errors.Join(err, ctx.Err())
}

func recoverForwarding(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	input, err := forwarding.ReadInputs(ctx, args[4])
	defer func() { returned = errors.Join(returned, input.Close()) }()
	if err != nil {
		return err
	}
	if err := input.IndependentPaths(args[0], args[1]); err != nil {
		return err
	}
	candidate, err := enrollment.ReadInfrastructureCandidate(ctx, args[0])
	if err != nil {
		return err
	}
	get := func(name string) []byte { raw, _ := candidate.File(name); return raw }
	metadata, err := release.StaticMetadataFiles(get("root.json"), get("timestamp.json"), get("snapshot.json"), get("targets.json"))
	if err != nil {
		return err
	}
	v, err := release.OpenRetained(args[1])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	authorization, err := infrastructure.AuthenticateForwardingCandidate(ctx, v, candidate, release.Inputs{
		RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata,
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	if err := forwarding.Recover(v, authorization, input, args[5], args[0], args[1]); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-recovery-stopped\",\"started\":false}\n")
	return err
}

func superviseForwardingTransition(parent context.Context, args []string, output io.Writer) error {
	if len(args) < 6 || args[0] != "--stop-after-record" || args[2] != "--timeout-ms" || args[4] != "--" {
		return forwarding.ErrInput
	}
	boundary, boundaryErr := strconv.ParseUint(args[1], 10, 32)
	milliseconds, timeoutErr := strconv.ParseUint(args[3], 10, 32)
	if boundaryErr != nil || timeoutErr != nil || strconv.FormatUint(boundary, 10) != args[1] || strconv.FormatUint(milliseconds, 10) != args[3] || milliseconds == 0 || milliseconds > 120000 {
		return forwarding.ErrInput
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(milliseconds)*time.Millisecond)
	defer cancel()
	cut, err := forwarding.RunTransition(ctx, args[5:], uint32(boundary))
	if err != nil || !cut {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-original-cut-completed\",\"recovered\":false}\n")
	return err
}

func replaceForwarding(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := forwarding.TransitionContext(parent, 2*time.Minute)
	defer cancel()
	input, err := forwarding.ReadInputs(ctx, args[4])
	defer func() { returned = errors.Join(returned, input.Close()) }()
	if err != nil {
		return err
	}
	completed, err := forwarding.ReadCompleted(ctx, args[5])
	if err == nil {
		err = completed.IndependentPaths(args[0], args[1])
	}
	if err = errors.Join(err, completed.Close(), input.IndependentPaths(args[0], args[1])); err != nil {
		return err
	}
	candidate, err := enrollment.ReadInfrastructureCandidate(ctx, args[0])
	if err != nil {
		return err
	}
	get := func(name string) []byte { raw, _ := candidate.File(name); return raw }
	metadata, err := release.StaticMetadataFiles(get("root.json"), get("timestamp.json"), get("snapshot.json"), get("targets.json"))
	if err != nil {
		return err
	}
	v, err := release.OpenRetained(args[1])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	authorization, err := infrastructure.AuthenticateForwardingCandidate(ctx, v, candidate, release.Inputs{
		RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata,
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	if err := forwarding.Replace(v, authorization, input, args[5], args[0], args[1]); err != nil {
		return err
	}
	if err := errors.Join(input.Close(), v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-replacement-selected-stopped\",\"started\":false}\n")
	return err
}

func stopForwarding(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := forwarding.TransitionContext(parent, 2*time.Minute)
	defer cancel()
	// Refuse an unbound history path before opening its mutable Release owner.
	completed, err := forwarding.ReadCompleted(ctx, args[1])
	if err == nil {
		err = completed.CheckHistoryPath(args[0])
	}
	if err = errors.Join(err, completed.Close()); err != nil {
		return err
	}
	v, err := release.OpenRetained(args[0])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	done := make(chan error, 1)
	go func() { done <- forwarding.Stop(ctx, v, args[1], args[0]) }()
	timer := time.NewTimer(2*time.Minute + 5*time.Second)
	defer timer.Stop()
	select {
	case err = <-done:
	case <-timer.C:
		_, noticeErr := io.WriteString(output, "{\"outcome\":\"forwarding-stop-completion-unavailable\"}\n")
		err = errors.Join(<-done, noticeErr)
	}
	if err = errors.Join(err, v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-original-stop-completed\",\"restart_admitted\":false}\n")
	return err
}

func inspectForwardingStart(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	completed, err := forwarding.ReadCompleted(ctx, args[1])
	if err == nil {
		err = completed.CheckHistoryPath(args[0])
	}
	if err = errors.Join(err, completed.Close()); err != nil {
		return err
	}
	v, err := release.OpenRetained(args[0])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	started, err := forwarding.ReadStarted(ctx, v, args[1], args[0])
	if err == nil {
		err = started.Check()
	}
	if err = errors.Join(err, started.Close(), v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-start-history-verified\",\"runtime_authority\":false,\"retirement_proved\":false}\n")
	return errors.Join(err, ctx.Err())
}

type forwardingStartOperation uint8

const (
	forwardingInitialStart forwardingStartOperation = iota
	forwardingRestart
	forwardingSelectedStart
)

func startInitialForwarding(parent context.Context, args []string, output io.Writer) error {
	return startForwardingGeneration(parent, args, output, forwardingInitialStart)
}

func startForwardingGeneration(parent context.Context, args []string, output io.Writer, operation forwardingStartOperation) (returned error) {
	if operation != forwardingInitialStart && operation != forwardingRestart && operation != forwardingSelectedStart {
		return forwarding.ErrInput
	}
	ctx, cancel := forwarding.TransitionContext(parent, 2*time.Minute)
	defer cancel()
	// Admission of independent paths precedes opening the mutable Release root.
	// Root Start then reopens its own complete initial observation and repeats
	// the same checks. No live creator/inspection handle is handed to runtime.
	completed, err := forwarding.ReadCompleted(ctx, args[4])
	if err == nil {
		err = completed.IndependentPaths(args[0], args[1])
	}
	if err = errors.Join(err, completed.Close()); err != nil {
		return err
	}
	candidate, err := enrollment.ReadInfrastructureCandidate(ctx, args[0])
	if err != nil {
		return err
	}
	get := func(name string) []byte { raw, _ := candidate.File(name); return raw }
	metadata, err := release.StaticMetadataFiles(get("root.json"), get("timestamp.json"), get("snapshot.json"), get("targets.json"))
	if err != nil {
		return err
	}
	v, err := release.OpenRetained(args[1])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	authorization, err := infrastructure.AuthenticateForwardingCandidate(ctx, v, candidate, release.Inputs{
		RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata,
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	// A lost original native join may outlive the admission deadline. Report
	// unavailability once and continue retaining this actual operation to join.
	done := make(chan error, 1)
	go func() {
		if operation == forwardingSelectedStart {
			done <- forwarding.StartSelected(ctx, v, authorization, args[4], args[0], args[1])
		} else if operation == forwardingRestart {
			done <- forwarding.Restart(ctx, v, authorization, args[4], args[0], args[1])
		} else {
			done <- forwarding.StartInitial(ctx, v, authorization, args[4], args[0], args[1])
		}
	}()
	timer := time.NewTimer(2*time.Minute + 5*time.Second)
	defer timer.Stop()
	select {
	case err = <-done:
	case <-timer.C:
		_, noticeErr := io.WriteString(output, "{\"outcome\":\"forwarding-start-completion-unavailable\"}\n")
		err = errors.Join(<-done, noticeErr)
	}
	if err = errors.Join(err, v.Close(), ctx.Err()); err != nil {
		return err
	}
	outcome := "{\"outcome\":\"forwarding-initial-start-completed\",\"runtime_authority\":false}\n"
	if operation == forwardingRestart {
		outcome = "{\"outcome\":\"forwarding-same-generation-restart-completed\",\"runtime_authority\":false}\n"
	}
	if operation == forwardingSelectedStart {
		outcome = "{\"outcome\":\"forwarding-selected-generation-start-completed\",\"runtime_authority\":false}\n"
	}
	_, err = io.WriteString(output, outcome)
	return err
}

func inspectInitialForwarding(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	completed, err := forwarding.ReadCompleted(ctx, args[4])
	defer func() { returned = errors.Join(returned, completed.Close()) }()
	if err != nil {
		return err
	}
	if err := completed.IndependentPaths(args[0], args[1]); err != nil {
		return err
	}
	candidate, err := enrollment.ReadInfrastructureCandidate(ctx, args[0])
	if err != nil {
		return err
	}
	get := func(name string) []byte { raw, _ := candidate.File(name); return raw }
	metadata, err := release.StaticMetadataFiles(get("root.json"), get("timestamp.json"), get("snapshot.json"), get("targets.json"))
	if err != nil {
		return err
	}
	v, err := release.OpenRetained(args[1])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	authorization, err := infrastructure.AuthenticateForwardingCandidate(ctx, v, candidate, release.Inputs{
		RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata,
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()},
	})
	if err != nil {
		return err
	}
	if err := completed.CheckAuthorization(v, authorization); err != nil {
		return err
	}
	if err := errors.Join(completed.Close(), v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-initial-history-verified\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func installInitialForwarding(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := forwarding.TransitionContext(parent, 2*time.Minute)
	defer cancel()
	input, err := forwarding.ReadInputs(ctx, args[5])
	defer func() { returned = errors.Join(returned, input.Close()) }()
	if err != nil {
		return err
	}
	if err = input.IndependentPaths(args[0], args[2]); err != nil {
		return err
	}
	bundle, err := enrollment.VerifyInfrastructure(ctx, args[0], args[1])
	if err != nil {
		return err
	}
	facts, ok := bundle.Facts()
	if !ok || facts.Invocation != "forwarding" {
		return forwarding.ErrBinding
	}
	get := func(name string) []byte { raw, _ := bundle.File(name); return raw }
	metadata, err := release.StaticMetadataFiles(get("root.json"), get("timestamp.json"), get("snapshot.json"), get("targets.json"))
	if err != nil {
		return err
	}
	v, err := release.Open(args[2])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	authorization, err := infrastructure.AuthenticateForwardingInitial(ctx, v, bundle, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[3], Network: args[4], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	if err = forwarding.InstallInitialStopped(v, authorization, input, args[0], args[2]); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"forwarding-initial-stopped-completed\",\"started\":false}\n")
	return errors.Join(err, ctx.Err())
}
