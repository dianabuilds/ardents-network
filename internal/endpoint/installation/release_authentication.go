package installation

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

const programTarget = "ardents/linux-amd64/endpoint"
const generationTarget = "ardents/linux-amd64/protected-endpoint"

// Authorization retains the two opaque fresh Release proofs after checking
// their coherence and actual generation bytes. Its zero value grants nothing.
// It is not serializable into a reusable installation or restart authority.
type Authorization struct {
	program, generation release.Authorization
	descriptor          []byte
	resources           map[string][]byte
}

// Targets returns the executable proof and then the generation proof. The
// executable replacement owner must receive the executable proof alone.
func (authorization Authorization) Targets() (release.Authorization, release.Authorization) {
	return authorization.program, authorization.generation
}

// Authenticate evaluates both targets against one exclusively held verifier.
// It freezes all supplied bytes before the first evaluation, which can advance
// durable Release floors. A subsequent refusal retains those floors; no
// participant, filesystem installation or manager effect is performed here.
func Authenticate(ctx context.Context, verifier *release.Verifier, enrolled enrollment.Verified) (Authorization, error) {
	return authenticateInputs(ctx, verifier, enrolled.Inputs, enrolled.ProtectedDescriptor, enrolled.ProtectedFiles)
}

func authenticateInputs(ctx context.Context, verifier *release.Verifier, input release.Inputs, descriptor []byte, resources map[string][]byte) (Authorization, error) {
	if verifier == nil {
		return Authorization{}, errors.New("protected installation requires a Release verifier")
	}
	if ctx == nil {
		return Authorization{}, errors.New("protected installation context is absent")
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	input.RootBytes = bytes.Clone(input.RootBytes)
	input.Artifact = bytes.Clone(input.Artifact)
	input.Files = cloneFiles(input.Files)
	descriptor = bytes.Clone(descriptor)
	resources = cloneFiles(resources)
	if input.TargetPath != programTarget || input.Local.Platform != "linux-amd64" || input.Local.Architecture != "amd64" {
		return Authorization{}, errors.New("protected installation requires the linux-amd64 Endpoint target")
	}
	var facts struct {
		ReleaseIdentity string `json:"release_identity"`
		ReleaseVersion  int64  `json:"release_version"`
		Platform        string `json:"platform"`
	}
	if err := json.Unmarshal(descriptor, &facts); err != nil {
		return Authorization{}, errors.New("protected installation lacks a generation descriptor")
	}
	if err := enrollment.ValidateProtectedGeneration(descriptor, resources, facts.ReleaseIdentity); err != nil {
		return Authorization{}, err
	}
	if facts.ReleaseIdentity == "" {
		return Authorization{}, errors.New("protected generation release identity is empty")
	}
	if !bytes.Equal(input.Artifact, resources["ardents-linux-amd64"]) {
		return Authorization{}, errors.New("protected generation Endpoint differs from the enrolled executable")
	}
	program := verifier.Evaluate(ctx, input)
	programProof, ok := program.Authorization()
	if !ok {
		return Authorization{}, fmt.Errorf("protected executable Release refused: %s", program.Outcome)
	}
	input.TargetPath, input.Artifact = generationTarget, descriptor
	generation := verifier.Evaluate(ctx, input)
	generationProof, ok := generation.Authorization()
	if !ok {
		return Authorization{}, fmt.Errorf("protected generation Release refused: %s", generation.Outcome)
	}
	if err := matchTargets(program, generation, facts.ReleaseIdentity, facts.ReleaseVersion, facts.Platform); err != nil {
		return Authorization{}, err
	}
	programDigest, generationDigest := sha256.Sum256(resources["ardents-linux-amd64"]), sha256.Sum256(descriptor)
	if !bytes.Equal(program.Digest, programDigest[:]) || !bytes.Equal(generation.Digest, generationDigest[:]) {
		return Authorization{}, errors.New("protected targets do not bind the actual generation bytes")
	}
	if err := ctx.Err(); err != nil {
		return Authorization{}, err
	}
	return Authorization{program: programProof, generation: generationProof, descriptor: descriptor, resources: resources}, nil
}

func cloneFiles(input map[string][]byte) map[string][]byte {
	result := make(map[string][]byte, len(input))
	for name, contents := range input {
		result[name] = bytes.Clone(contents)
	}
	return result
}

func matchTargets(program, generation release.Decision, identity string, version int64, platform string) error {
	if program.Path != programTarget || generation.Path != generationTarget ||
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
