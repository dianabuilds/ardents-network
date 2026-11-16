package main

import (
	"context"
	"errors"
	"io"
	"runtime"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// Initial infrastructure verification retains actual executable Enrollment
// provenance and the original Release lease. Its report grants no installation.
func runInitialInfrastructureRelease(ctx context.Context, args []string, out io.Writer) error {
	if ctx == nil || out == nil || len(args) != 6 || args[0] != "authenticate-initial" || runtime.GOARCH != "amd64" {
		return releaseartifact.ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !independentReleasePaths(args[1], args[3]) {
		return releaseartifact.ErrBinding
	}
	bundle, err := enrollment.VerifyInfrastructure(ctx, args[1], args[2])
	if err != nil {
		return err
	}
	facts, ok := bundle.Facts()
	if !ok {
		return releaseartifact.ErrInput
	}
	input, err := releaseartifact.ProgramInputs(bundle)
	if err != nil {
		return err
	}
	input.Local = release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[4], Network: args[5], RefTime: time.Now().UTC()}
	v, err := release.Open(args[3])
	if err != nil {
		return err
	}
	var authorization interface {
		Check(context.Context, *release.Verifier) error
		Targets() (release.Authorization, release.Authorization)
		InitialFacts() (enrollment.InfrastructureFacts, bool)
	}
	switch facts.Invocation {
	case "source":
		authorization, err = releaseartifact.AuthenticateInitial(ctx, v, bundle, input)
	case "forwarding":
		authorization, err = releaseartifact.AuthenticateForwardingInitial(ctx, v, bundle, input)
	case "join":
		authorization, err = releaseartifact.AuthenticateJoinInitial(ctx, v, bundle, input)
	case "descriptor":
		authorization, err = releaseartifact.AuthenticateDescriptorInitial(ctx, v, bundle, input)
	case "introduction":
		authorization, err = releaseartifact.AuthenticateIntroductionInitial(ctx, v, bundle, input)
	default:
		err = releaseartifact.ErrInput
	}
	if err == nil {
		err = authorization.Check(ctx, v)
		program, generation := authorization.Targets()
		_, programOK := program.AcceptedDecision()
		_, generationOK := generation.AcceptedDecision()
		_, initialOK := authorization.InitialFacts()
		if !programOK || !generationOK || !initialOK {
			err = errors.Join(err, releaseartifact.ErrAuthorization)
		}
	}
	err = errors.Join(err, v.Close(), ctx.Err())
	if err != nil {
		return err
	}
	return writeSourceCommandOutcome(ctx, out, "release.authenticate-initial", "authenticated-"+facts.Invocation+"-artifact")
}
