//go:build linux || darwin

package main

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// The retained candidate path checks one fixed role and never conveys an
// installation, first-pin or runtime right to the caller.
func runNodeRelease(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) != 6 {
		return enrollmentReport(out, "invalid-input", 2)
	}
	if ctx.Err() != nil {
		return enrollmentReport(out, "release-canceled", 130)
	}
	role := strings.TrimSuffix(strings.TrimPrefix(args[0], "authenticate-"), "-candidate")
	switch role {
	case "forwarding", "join", "descriptor", "introduction":
	default:
		return enrollmentReport(out, "invalid-input", 2)
	}
	request, invalid := parseSourceRelease(append([]string{"authenticate-candidate"}, args[1:]...))
	if invalid != "" {
		return enrollmentReport(out, invalid, 2)
	}
	candidate, err := enrollment.ReadInfrastructureCandidate(ctx, request.bundle)
	if err != nil {
		return enrollmentReport(out, "infrastructure-inventory-refused", 1)
	}
	facts, ok := candidate.Facts()
	if !ok || facts.Invocation != role {
		return enrollmentReport(out, "infrastructure-artifact-refused", 1)
	}
	input, err := releaseartifact.ProgramInputs(candidate)
	if err != nil {
		return enrollmentReport(out, "infrastructure-metadata-refused", 1)
	}
	input.Local = release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: request.environment, Network: request.network, RefTime: request.reference}
	v, err := release.OpenRetained(request.history)
	if err != nil {
		return enrollmentReport(out, "infrastructure-release-unavailable", 1)
	}
	if err := sameReleaseMetadata(ctx, v, input); err != nil {
		_ = v.Close()
		return enrollmentReport(out, "infrastructure-authentication-refused", 1)
	}
	var a interface {
		Check(context.Context, *release.Verifier) error
		Targets() (release.Authorization, release.Authorization)
		InitialFacts() (enrollment.InfrastructureFacts, bool)
		Program() []byte
		Descriptor() []byte
		Template() []byte
	}
	switch role {
	case "forwarding":
		a, err = releaseartifact.AuthenticateForwardingCandidate(ctx, v, candidate, input)
	case "join":
		a, err = releaseartifact.AuthenticateJoinCandidate(ctx, v, candidate, input)
	case "descriptor":
		a, err = releaseartifact.AuthenticateDescriptorCandidate(ctx, v, candidate, input)
	case "introduction":
		a, err = releaseartifact.AuthenticateIntroductionCandidate(ctx, v, candidate, input)
	}
	var program, generation release.Decision
	if err == nil {
		err = a.Check(ctx, v)
		p, g := a.Targets()
		var programOK, generationOK bool
		program, programOK = p.AcceptedDecision()
		generation, generationOK = g.AcceptedDecision()
		_, hasInitial := a.InitialFacts()
		if !programOK || !generationOK || hasInitial {
			err = errors.Join(err, releaseartifact.ErrAuthorization)
		}
	}
	err = errors.Join(err, v.Close(), ctx.Err())
	if err != nil {
		return enrollmentReport(out, "infrastructure-authentication-refused", 1)
	}
	if writeInfrastructureReleaseReceipt(ctx, out, "authenticated-"+role+"-artifact", a, program, generation) != nil {
		return 1
	}
	return 0
}
