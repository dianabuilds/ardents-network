package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
	"github.com/dianabuilds/ardents-network/internal/release"
)

const protectedEnrollmentProgramTarget = "ardents/linux-amd64/endpoint"
const protectedEnrollmentGenerationTarget = "ardents/linux-amd64/protected-endpoint"

// protectedEnrollmentAuthorization retains the two opaque fresh Release proofs after checking
// their coherence and actual generation bytes. Its zero value grants nothing.
// This command-local pair supplies no native installation or restart authority.
type protectedEnrollmentAuthorization struct {
	program, generation release.Authorization
}

// Targets returns the executable proof and then the generation proof. The
// executable replacement owner must receive the executable proof alone.
func (authorization protectedEnrollmentAuthorization) Targets() (release.Authorization, release.Authorization) {
	return authorization.program, authorization.generation
}

// authenticateProtectedEnrollment evaluates both targets against one exclusively held verifier.
// It freezes all supplied bytes before the first evaluation, which can advance
// durable Release floors. A subsequent refusal retains those floors; no
// participant, filesystem installation or manager effect is performed here.
func authenticateProtectedEnrollment(ctx context.Context, verifier *release.Verifier, enrolled enrollment.Verified) (protectedEnrollmentAuthorization, error) {
	return authenticateProtectedEnrollmentInputs(ctx, verifier, enrolled.Inputs, enrolled.ProtectedDescriptor, enrolled.ProtectedFiles)
}

func authenticateProtectedEnrollmentInputs(ctx context.Context, verifier *release.Verifier, input release.Inputs, descriptor []byte, resources map[string][]byte) (protectedEnrollmentAuthorization, error) {
	if verifier == nil {
		return protectedEnrollmentAuthorization{}, errors.New("protected enrollment requires a Release verifier")
	}
	if ctx == nil {
		return protectedEnrollmentAuthorization{}, errors.New("protected enrollment context is absent")
	}
	if err := ctx.Err(); err != nil {
		return protectedEnrollmentAuthorization{}, err
	}
	input.RootBytes = bytes.Clone(input.RootBytes)
	input.Artifact = bytes.Clone(input.Artifact)
	input.Files = cloneProtectedEnrollmentFiles(input.Files)
	descriptor = bytes.Clone(descriptor)
	resources = cloneProtectedEnrollmentFiles(resources)
	if input.TargetPath != protectedEnrollmentProgramTarget || input.Local.Platform != "linux-amd64" || input.Local.Architecture != "amd64" {
		return protectedEnrollmentAuthorization{}, errors.New("protected enrollment requires the linux-amd64 Endpoint target")
	}
	var facts struct {
		ReleaseIdentity string `json:"release_identity"`
		ReleaseVersion  int64  `json:"release_version"`
		Platform        string `json:"platform"`
	}
	if err := json.Unmarshal(descriptor, &facts); err != nil {
		return protectedEnrollmentAuthorization{}, errors.New("protected enrollment lacks a generation descriptor")
	}
	if err := enrollment.ValidateProtectedGeneration(descriptor, resources, facts.ReleaseIdentity); err != nil {
		return protectedEnrollmentAuthorization{}, err
	}
	if facts.ReleaseIdentity == "" {
		return protectedEnrollmentAuthorization{}, errors.New("protected generation release identity is empty")
	}
	if !bytes.Equal(input.Artifact, resources["ardents-linux-amd64"]) {
		return protectedEnrollmentAuthorization{}, errors.New("protected generation Endpoint differs from the enrolled executable")
	}
	program := verifier.Evaluate(ctx, input)
	programProof, ok := program.Authorization()
	if !ok {
		return protectedEnrollmentAuthorization{}, fmt.Errorf("protected executable Release refused: %s", program.Outcome)
	}
	input.TargetPath, input.Artifact = protectedEnrollmentGenerationTarget, descriptor
	generation := verifier.Evaluate(ctx, input)
	generationProof, ok := generation.Authorization()
	if !ok {
		return protectedEnrollmentAuthorization{}, fmt.Errorf("protected generation Release refused: %s", generation.Outcome)
	}
	if err := matchProtectedEnrollmentTargets(program, generation, facts.ReleaseIdentity, facts.ReleaseVersion, facts.Platform); err != nil {
		return protectedEnrollmentAuthorization{}, err
	}
	programDigest, generationDigest := sha256.Sum256(resources["ardents-linux-amd64"]), sha256.Sum256(descriptor)
	if !bytes.Equal(program.Digest, programDigest[:]) || !bytes.Equal(generation.Digest, generationDigest[:]) {
		return protectedEnrollmentAuthorization{}, errors.New("protected targets do not bind the actual generation bytes")
	}
	if err := ctx.Err(); err != nil {
		return protectedEnrollmentAuthorization{}, err
	}
	return protectedEnrollmentAuthorization{program: programProof, generation: generationProof}, nil
}

func cloneProtectedEnrollmentFiles(input map[string][]byte) map[string][]byte {
	result := make(map[string][]byte, len(input))
	for name, contents := range input {
		result[name] = bytes.Clone(contents)
	}
	return result
}

func matchProtectedEnrollmentTargets(program, generation release.Decision, identity string, version int64, platform string) error {
	if program.Path != protectedEnrollmentProgramTarget || generation.Path != protectedEnrollmentGenerationTarget ||
		program.ReleaseIdentity != identity || generation.ReleaseIdentity != identity ||
		program.ReleaseVersion != version || generation.ReleaseVersion != version ||
		program.Platform != platform || generation.Platform != platform ||
		program.Architecture != generation.Architecture || program.Environment != generation.Environment ||
		program.Network != generation.Network || !program.ReferenceTime.Equal(generation.ReferenceTime) ||
		program.Floors.TargetsVersion != generation.Floors.TargetsVersion ||
		!bytes.Equal(program.Floors.TargetsDigest, generation.Floors.TargetsDigest) {
		return errors.New("protected Release targets are not one coherent generation")
	}
	return nil
}
