//go:build linux || darwin

package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// sourceReleaseInput contains only the parsed and path-checked command inputs.
// It is not Enrollment or Release authority.
type sourceReleaseInput struct {
	bundle, history, environment, network string
	reference                             time.Time
}

func parseSourceRelease(args []string) (sourceReleaseInput, string) {
	var input sourceReleaseInput
	if len(args) == 0 {
		return input, "invalid-input"
	}
	if args[0] != "authenticate-candidate" || len(args) != 6 {
		return input, "invalid-input"
	}
	ref, err := time.Parse(time.RFC3339, args[len(args)-1])
	if err != nil {
		return sourceReleaseInput{}, "invalid-input"
	}
	input.bundle, input.history = args[1], args[2]
	input.environment, input.network, input.reference = args[3], args[4], ref.UTC()
	if !independentInfrastructureHistory(input.bundle, input.history) {
		return sourceReleaseInput{}, "infrastructure-paths-refused"
	}
	return input, ""
}

// openSourceRelease retains the original verifier on every successful open.
// The caller joins it after its own borrowers and must not treat a public result
// as an installation or runtime authorization.
func openSourceRelease(ctx context.Context, request sourceReleaseInput) (releaseartifact.Authorization, *release.Verifier, string, error) {
	local := release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: request.environment, Network: request.network, RefTime: request.reference}
	c, err := enrollment.ReadInfrastructureCandidate(ctx, request.bundle)
	if err != nil {
		return releaseartifact.Authorization{}, nil, "infrastructure-inventory-refused", err
	}
	program, _ := c.File(releaseartifact.ProgramName)
	descriptor, _ := c.File(releaseartifact.DescriptorName)
	template, _ := c.File(releaseartifact.TemplateName)
	d, err := releaseartifact.ValidateResources(descriptor, program, template)
	f, _ := c.Facts()
	if err != nil || d.ReleaseIdentity != f.ReleaseIdentity || d.ReleaseVersion != f.ReleaseVersion {
		return releaseartifact.Authorization{}, nil, "infrastructure-artifact-refused", err
	}
	input, err := releaseartifact.ProgramInputs(c)
	if err != nil {
		return releaseartifact.Authorization{}, nil, "infrastructure-metadata-refused", err
	}
	input.Local = local
	v, err := release.OpenRetained(request.history)
	if err != nil {
		return releaseartifact.Authorization{}, nil, "infrastructure-release-unavailable", err
	}
	if err := sameReleaseMetadata(ctx, v, input); err != nil {
		return releaseartifact.Authorization{}, v, "", err
	}
	a, err := releaseartifact.AuthenticateCandidate(ctx, v, c, input)
	if err == nil {
		err = a.Check(ctx, v)
	}
	return a, v, "", err
}

// Immutable distribution and mutable Release history cannot contain one another.
// Direct ancestor checks also refuse a differently spelled symlink alias before
// Release opens its independently owned native lease or creates any directory.
func independentInfrastructureHistory(distribution, history string) bool {
	if !absoluteAdmissionPath(distribution) || !absoluteAdmissionPath(history) {
		return false
	}
	contains := func(parent, child string) bool {
		rel, err := filepath.Rel(parent, child)
		return err == nil && (rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))))
	}
	if contains(distribution, history) || contains(history, distribution) {
		return false
	}
	for p := history; ; p = filepath.Dir(p) {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return false
		}
		if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
			return false
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return true
}

// The public receipt identifies the exact authenticated bytes; it carries no
// live authorization or permission to install or run them.
func writeInfrastructureReleaseReceipt(ctx context.Context, out io.Writer, a releaseartifact.Authorization, program, generation release.Decision) error {
	receipt, err := json.Marshal(struct {
		Outcome           string          `json:"outcome"`
		ProgramOutcome    release.Outcome `json:"program_outcome"`
		GenerationOutcome release.Outcome `json:"generation_outcome"`
		ProgramSHA256     string          `json:"program_sha256"`
		DescriptorSHA256  string          `json:"descriptor_sha256"`
		TemplateSHA256    string          `json:"template_sha256"`
	}{"authenticated-source-artifact", program.Outcome, generation.Outcome, fmt.Sprintf("%x", sha256.Sum256(a.Program())), fmt.Sprintf("%x", sha256.Sum256(a.Descriptor())), fmt.Sprintf("%x", sha256.Sum256(a.Template()))})
	if err != nil {
		return err
	}
	return errors.Join(writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')), ctx.Err())
}
