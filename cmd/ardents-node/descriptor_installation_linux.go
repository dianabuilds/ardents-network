package main

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/descriptor"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// Inspection retains its own original delivery and read-only manager. Neither
// its result nor prepared public bytes admit installation or runtime effects.
func runDescriptorInstallation(parent context.Context, args []string, output io.Writer) error {
	if parent != nil && output != nil && len(args) == 7 && args[0] == "replace" {
		return replaceDescriptorGeneration(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 2 && args[0] == "start-installed" {
		return runInstalledDescriptor(parent, args[1], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "start-initial" {
		return startDescriptorGeneration(parent, args[1:], output, false)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "restart" {
		return startDescriptorGeneration(parent, args[1:], output, true)
	}
	if parent != nil && output != nil && len(args) == 3 && args[0] == "stop" {
		return stopDescriptor(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 3 && args[0] == "inspect-started-history" {
		return inspectStartedDescriptor(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 7 && args[0] == "install-initial-stopped" {
		return installInitialDescriptor(parent, args[1:], output)
	}
	if parent != nil && output != nil && len(args) == 6 && args[0] == "inspect-initial-history" {
		return inspectInitialDescriptor(parent, args[1:], output)
	}
	if parent == nil || output == nil || len(args) != 2 || args[0] != "inspect-inputs" {
		return errors.New("usage: ardents-node descriptor inspect-inputs REQUEST | install-initial-stopped DISTRIBUTION PIN RELEASE-HISTORY ENVIRONMENT NETWORK REQUEST | inspect-initial-history DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING | start-initial DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING | stop RELEASE-HISTORY BINDING | inspect-started-history RELEASE-HISTORY BINDING | restart DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING | replace DISTRIBUTION RELEASE-HISTORY ENVIRONMENT NETWORK BINDING REQUEST")
	}
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	input, err := descriptor.ReadInputs(ctx, args[1])
	if err == nil {
		err = descriptor.InspectInitialInputs(input)
	}
	if err = errors.Join(err, input.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"descriptor-inputs-preflight-completed\",\"installed\":false}\n")
	return errors.Join(err, ctx.Err())
}

func installInitialDescriptor(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	input, err := descriptor.ReadInputs(ctx, args[5])
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
	if !ok || facts.Invocation != "descriptor" {
		return descriptor.ErrBinding
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
	authorization, err := infrastructure.AuthenticateDescriptorInitial(ctx, v, bundle, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[3], Network: args[4], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	if err := descriptor.InstallInitialStopped(v, authorization, input, args[0], args[2]); err != nil {
		return err
	}
	_, err = io.WriteString(output, "{\"outcome\":\"descriptor-initial-stopped-completed\",\"started\":false}\n")
	return errors.Join(err, ctx.Err())
}

func inspectInitialDescriptor(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	completed, err := descriptor.ReadCompleted(ctx, args[4])
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
	authorization, err := infrastructure.AuthenticateDescriptorCandidate(ctx, v, candidate, release.Inputs{
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
	_, err = io.WriteString(output, "{\"outcome\":\"descriptor-initial-history-verified\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func startDescriptorGeneration(parent context.Context, args []string, output io.Writer, restart bool) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	preflight, err := descriptor.ReadCompleted(ctx, args[4])
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
	a, err := infrastructure.AuthenticateDescriptorCandidate(ctx, v, candidate, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	start := descriptor.StartInitial
	if restart {
		start = descriptor.Restart
	}
	if err := start(ctx, v, a, args[4], args[0], args[1]); err != nil {
		return err
	}
	outcome := "descriptor-first-start-completed"
	if restart {
		outcome = "descriptor-restart-completed"
	}
	_, err = io.WriteString(output, "{\"outcome\":\""+outcome+"\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}

func stopDescriptor(parent context.Context, args []string, out io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	completed, err := descriptor.ReadCompleted(ctx, args[1])
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
	go func() { done <- descriptor.Stop(ctx, v, args[1], args[0]) }()
	timer := time.NewTimer(2*time.Minute + 5*time.Second)
	defer timer.Stop()
	select {
	case err = <-done:
	case <-timer.C:
		_, notice := io.WriteString(out, "{\"outcome\":\"descriptor-stop-completion-unavailable\"}\n")
		err = errors.Join(<-done, notice)
	}
	if err = errors.Join(err, v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(out, "{\"outcome\":\"descriptor-original-stop-completed\",\"restart_admitted\":false}\n")
	return err
}

func inspectStartedDescriptor(parent context.Context, args []string, out io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	c, err := descriptor.ReadCompleted(ctx, args[1])
	if err == nil {
		err = c.CheckHistoryPath(args[0])
	}
	if err = errors.Join(err, c.Close()); err != nil {
		return err
	}
	v, err := release.OpenRetained(args[0])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, v.Close()) }()
	h, err := descriptor.ReadStarted(ctx, v, args[1], args[0])
	if err == nil {
		err = h.Check()
	}
	if err = errors.Join(err, h.Close(), v.Close(), ctx.Err()); err != nil {
		return err
	}
	_, err = io.WriteString(out, "{\"outcome\":\"descriptor-started-history-verified\",\"runtime_authority\":false}\n")
	return err
}

func replaceDescriptorGeneration(parent context.Context, args []string, output io.Writer) (returned error) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	preflight, err := descriptor.ReadCompleted(ctx, args[4])
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
	a, err := infrastructure.AuthenticateDescriptorCandidate(ctx, v, candidate, release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata, Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[2], Network: args[3], RefTime: time.Now().UTC()}})
	if err != nil {
		return err
	}
	input, err := descriptor.ReadInputs(ctx, args[5])
	if err != nil {
		return err
	}
	defer func() { returned = errors.Join(returned, input.Close()) }()
	if err = descriptor.Replace(v, a, input, args[4], args[0], args[1]); err != nil {
		return err
	}
	outcome := "descriptor-replacement-stopped-completed"

	_, err = io.WriteString(output, "{\"outcome\":\""+outcome+"\",\"runtime_authority\":false}\n")
	return errors.Join(err, ctx.Err())
}
