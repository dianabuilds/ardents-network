package main

import (
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/introduction"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"io"
	"time"
)

func installInitialIntroduction(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	input, err := introduction.ReadInputs(ctx, args[5])
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
	if !ok || facts.Invocation != "introduction" {
		return introduction.ErrBinding
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
	authorization, err := infrastructure.AuthenticateIntroductionInitial(ctx, v, bundle, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[3], Network: args[4], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	if err := introduction.InstallInitialStopped(v, authorization, input, args[0], args[2]); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"introduction-initial-stopped-completed\",\"started\":false}\n")
	return errors.Join(err, ctx.Err())
}

func inspectInitialIntroduction(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	completed, err := introduction.ReadCompleted(ctx, args[4])
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
	authorization, err := infrastructure.AuthenticateIntroductionCandidate(ctx, v, candidate, release.Inputs{
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
	_, err = io.WriteString(output, "{\"outcome\":\"introduction-initial-history-verified\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func startIntroductionGeneration(parent context.Context, args []string, output io.Writer, retained bool) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	if !retained {
		preflight, err := introduction.ReadCompleted(ctx, args[4])
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
	a, err := infrastructure.AuthenticateIntroductionCandidate(ctx, v, candidate, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	start := introduction.StartInitial
	if retained {
		start = introduction.Restart
	}
	if err := start(ctx, v, a, args[4], args[0], args[1]); err != nil {
		return err
	}
	outcome := "introduction-first-start-completed"
	if retained {
		outcome = "introduction-restart-completed"
	}
	_, err = io.WriteString(output, "{\"outcome\":\""+outcome+"\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func inspectStartedIntroduction(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	v, err := release.OpenRetained(args[0])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	history, err := introduction.ReadStarted(ctx, v, args[1], args[0])
	defer func() { returned = errors.Join(returned, history.Close()) }()
	if err != nil {
		return err
	}
	if err := errors.Join(history.Check(), history.Close(), v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"introduction-start-history-verified\",\"runtime_authority\":false,\"retirement_proved\":false}\n")
	return errors.Join(err, ctx.Err())
}

func stopIntroduction(parent context.Context, args []string, out io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	v, err := release.OpenRetained(args[0])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	done := make(chan error, 1)
	go func() { done <- introduction.Stop(ctx, v, args[1], args[0]) }()
	timer := time.NewTimer(2*time.Minute + 5*time.Second)
	defer timer.Stop()
	select {
	case err = <-done:
	case <-timer.C:
		_, notice := io.WriteString(out, "{\"outcome\":\"introduction-stop-completion-unavailable\"}\n")
		err = errors.Join(<-done, notice)
	}
	if err = errors.Join(err, v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(out, "{\"outcome\":\"introduction-original-stop-completed\",\"restart_admitted\":false}\n")
	return err
}

func inspectIntroductionRetirement(parent context.Context, args []string, out io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	v, err := release.OpenRetained(args[0])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	h, err := introduction.ReadStarted(ctx, v, args[1], args[0])
	if err == nil {
		err = h.CheckRetirement()
	}
	if err = errors.Join(err, h.Close(), v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(out, "{\"outcome\":\"introduction-retirement-verified\",\"restart_admitted\":false}\n")
	return err
}
