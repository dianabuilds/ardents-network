package main

import (
	"context"
	"errors"
	"io"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/join"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// JOIN uses its separate declaration and original input lifetime. It never
// enters the legacy Node dispatcher or borrows Forwarding/Source permission.
func runJoinInstallation(parent context.Context, args []string, output io.Writer) (returned error) {
	if parent == nil || output == nil || len(args) == 0 {
		return join.ErrInput
	}
	if args[0] == "supervise-transition" {
		return superviseJoinTransition(parent, args[1:], output)
	}
	coordinator, managed, err := join.OpenTransitionCoordinator(args[0])
	if err != nil {
		return err
	}
	if managed {
		parent = coordinator.Context()
		defer func() { returned = errors.Join(returned, coordinator.Close()) }()
	}
	if len(args) == 7 && args[0] == "recover" {
		return recoverJoinGeneration(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 2 && args[0] == "start-installed" {
		return runInstalledJoin(parent, args[1], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "start-initial" {
		return startJoinGeneration(parent, args[1:], output, false)
	}

	if parent != nil && output != nil && len(args) == 7 && args[0] == "replace" {
		return replaceJoinGeneration(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "restart" {
		return startJoinGeneration(parent, args[1:], output, true)
	}
	if parent != nil && output != nil && len(args) == 3 && args[0] == "stop" {
		return stopJoin(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "inspect-initial-history" {
		return inspectInitialJoin(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 7 && args[0] == "install-initial-stopped" {
		return installInitialJoin(parent, args[1:], output)
	}
	if parent == nil || output == nil || len(args) != 2 || args[0] != "inspect-inputs" {
		return errors.New("usage: ardents-node join inspect-inputs REQUEST | install-initial-stopped DISTRIBUTION PIN HISTORY ENVIRONMENT NETWORK REQUEST | inspect-initial-history DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING | replace DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING REQUEST")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	input, err := join.ReadInputs(ctx, args[1])
	if err == nil {
		err = join.InspectInitialInputs(input)
	}
	if err = errors.Join(err, input.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"join-inputs-preflight-completed\",\"installed\":false}\n")
	return errors.Join(err, ctx.Err())
}

func installInitialJoin(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	input, err := join.ReadInputs(ctx, args[5])
	defer func() { returned = errors.Join(returned, input.Close()) }()
	if err != nil {
		return err
	}
	if err := input.IndependentPaths(args[0], args[2]); err != nil {
		return err
	}
	bundle, err := enrollment.VerifyInfrastructure(ctx, args[0], args[1])
	if err != nil {
		return err
	}
	facts, ok := bundle.Facts()
	if !ok || facts.Invocation != "join" {
		return join.ErrBinding
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
	authorization, err := infrastructure.AuthenticateJoinInitial(ctx, v, bundle, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[3], Network: args[4], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	if err := join.InstallInitialStopped(v, authorization, input, args[0], args[2]); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"join-initial-stopped-completed\",\"started\":false}\n")
	return errors.Join(err, ctx.Err())
}

func inspectInitialJoin(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	completed, err := join.ReadCompleted(ctx, args[4])
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
	authorization, err := infrastructure.AuthenticateJoinCandidate(ctx, v, candidate, release.Inputs{
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
	_, err = io.WriteString(output, "{\"outcome\":\"join-initial-history-verified\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func startJoinGeneration(parent context.Context, args []string, output io.Writer, restart bool) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	preflight, err := join.ReadCompleted(ctx, args[4])
	defer func() { returned = errors.Join(returned, preflight.Close()) }()
	if err != nil {
		return err
	}
	if err := preflight.IndependentPaths(args[0], args[1]); err != nil {
		return err
	}
	if err := preflight.Close(); err != nil {
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
	a, err := infrastructure.AuthenticateJoinCandidate(ctx, v, candidate, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	start := join.StartInitial
	if restart {
		start = join.Restart
	}
	if err := start(ctx, v, a, args[4], args[0], args[1]); err != nil {
		return err
	}
	outcome := "join-first-start-completed"
	if restart {
		outcome = "join-restart-completed"
	}
	_, err = io.WriteString(output, "{\"outcome\":\""+outcome+"\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func stopJoin(parent context.Context, args []string, out io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	completed, err := join.ReadCompleted(ctx, args[1])
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
	go func() { done <- join.Stop(ctx, v, args[1], args[0]) }()
	timer := time.NewTimer(2*time.Minute + 5*time.Second)
	defer timer.Stop()
	select {
	case err = <-done:
	case <-timer.C:
		_, notice := io.WriteString(out, "{\"outcome\":\"join-stop-completion-unavailable\"}\n")
		err = errors.Join(<-done, notice)
	}
	if err = errors.Join(err, v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(out, "{\"outcome\":\"join-original-stop-completed\",\"restart_admitted\":false}\n")
	return err
}

func replaceJoinGeneration(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := join.TransitionContext(parent, 2*time.Minute)
	defer cancel()
	preflight, err := join.ReadCompleted(ctx, args[4])
	defer func() { returned = errors.Join(returned, preflight.Close()) }()
	if err != nil {
		return err
	}
	if err := preflight.IndependentPaths(args[0], args[1]); err != nil {
		return err
	}
	if err := preflight.Close(); err != nil {
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
	a, err := infrastructure.AuthenticateJoinCandidate(ctx, v, candidate, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	input, err := join.ReadInputs(ctx, args[5])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, input.Close()) }()
	if err = join.Replace(v, a, input, args[4], args[0], args[1]); err != nil {
		return err
	}
	outcome := "join-replacement-stopped-completed"

	_, err = io.WriteString(output, "{\"outcome\":\""+outcome+"\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func superviseJoinTransition(parent context.Context, args []string, output io.Writer) error {
	if len(args) < 6 || args[0] != "--stop-after-record" || args[2] != "--timeout-ms" || args[4] != "--" {
		return join.ErrInput
	}
	boundary, boundaryErr := strconv.ParseUint(args[1], 10, 32)
	milliseconds, timeoutErr := strconv.ParseUint(args[3], 10, 32)
	if boundaryErr != nil || timeoutErr != nil || strconv.FormatUint(boundary, 10) != args[1] || strconv.FormatUint(milliseconds, 10) != args[3] || milliseconds == 0 || milliseconds > 120000 {
		return join.ErrInput
	}
	ctx, cancel := context.WithTimeout(parent, time.Duration(milliseconds)*time.Millisecond)
	defer cancel()
	cut, err := join.RunTransition(ctx, args[5:], uint32(boundary))
	if err != nil || !cut {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"join-original-cut-completed\",\"recovered\":false}\n")
	return err
}

func recoverJoinGeneration(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := join.TransitionContext(parent, 2*time.Minute)
	defer cancel()
	input, err := join.ReadInputs(ctx, args[5])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, input.Close()) }()
	if err := input.CheckRecoveryPaths(args[4], args[0], args[1]); err != nil {
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
	a, err := infrastructure.AuthenticateJoinCandidate(ctx, v, candidate, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	if err = join.RecoverReplacement(v, a, input, args[4], args[0], args[1]); err != nil {
		return err
	}
	outcome := "join-recovery-stopped-completed"

	_, err = io.WriteString(output, "{\"outcome\":\""+outcome+"\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}
