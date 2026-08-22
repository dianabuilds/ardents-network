package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/update"
)

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}
func run(arguments []string, output io.Writer, errorOutput io.Writer) (runErr error) {
	if len(arguments) == 0 {
		return errors.New("usage: ardents-release <offline-import|apply-offline> [flags]")
	}
	operation := arguments[0]
	if operation != "offline-import" && operation != "apply-offline" {
		return fmt.Errorf("unknown subcommand %q", arguments[0])
	}
	flags := flag.NewFlagSet(operation, flag.ContinueOnError)
	flags.SetOutput(errorOutput)
	raw := &offlineImportFlags{}
	raw.register(flags)
	var updateRoot string
	if operation == "apply-offline" {
		flags.StringVar(&updateRoot, "update-root", "", "initialized Update Transaction root")
	}
	if err := flags.Parse(arguments[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("%s has unexpected positional arguments", operation)
	}
	if operation == "apply-offline" && updateRoot == "" {
		return errors.New("apply-offline requires -update-root")
	}
	inputs, err := raw.buildInputs()
	if err != nil {
		return err
	}
	verifier, err := release.Open(raw.stateRoot)
	if err != nil {
		return fmt.Errorf("open state root: %w", err)
	}
	defer func() { runErr = errors.Join(runErr, verifier.Close()) }()
	decision := verifier.Evaluate(context.Background(), inputs)
	if operation == "apply-offline" {
		if decision.Outcome != release.OutcomeReleaseAccepted {
			return errors.New("apply-offline requires a release-accepted decision")
		}
		authorization, ok := decision.Authorization()
		if !ok {
			return errors.New("apply-offline requires release authorization")
		}
		result, err := update.Apply(context.Background(), update.Request{
			UpdateRoot:    updateRoot,
			Authorization: authorization, Artifact: inputs.Artifact,
			Work: stoppedRuntime{}, SelfTest: offlineCandidateTest{},
		})
		if err != nil {
			return fmt.Errorf("apply-offline transaction failed in state %q with outcome %q: %w", result.State, result.Outcome, hiddenApplyError{err})
		}
		rendered, err := renderUpdateResult(result)
		if err != nil {
			return err
		}
		_, err = output.Write(rendered)
		return err
	}
	rendered, err := renderDecision(decision)
	if err != nil {
		return err
	}
	_, err = output.Write(rendered)
	return err
}

type hiddenApplyError struct{ cause error }

func (failure hiddenApplyError) Error() string { return "bounded internal failure" }
func (failure hiddenApplyError) Unwrap() error { return failure.cause }

type stoppedRuntime struct{}

func (stoppedRuntime) StopNewWork(ctx context.Context) error        { return ctx.Err() }
func (stoppedRuntime) Drain(ctx context.Context) error              { return ctx.Err() }
func (stoppedRuntime) StopNewAssignments(ctx context.Context) error { return ctx.Err() }
func (stoppedRuntime) DrainAssignments(ctx context.Context) error   { return ctx.Err() }
func (stoppedRuntime) RejoinOrWithdraw(ctx context.Context) error   { return ctx.Err() }

type offlineCandidateTest struct{}

func (offlineCandidateTest) Check(ctx context.Context, identity update.CandidateIdentity) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if identity.Generation == 0 || identity.TargetPath == "" || identity.Length <= 0 || identity.Digest == ([32]byte{}) || identity.Platform == "" || identity.Architecture == "" || identity.Environment == "" || identity.Network == "" {
		return errors.New("offline candidate identity is incomplete")
	}
	return nil
}
