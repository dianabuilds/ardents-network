//go:build linux

package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

// sourceReleaseInput contains only the parsed and path-checked command inputs.
// It is not Enrollment or Release authority.
type sourceReleaseInput struct {
	initial                                    bool
	bundle, pin, history, environment, network string
	reference                                  time.Time
}

func parseSourceRelease(args []string) (sourceReleaseInput, string) {
	var input sourceReleaseInput
	if len(args) == 0 {
		return input, "invalid-input"
	}
	input.initial = args[0] == "authenticate-initial"
	if (input.initial && len(args) != 7) || (!input.initial && (args[0] != "authenticate-candidate" || len(args) != 6)) {
		return sourceReleaseInput{}, "invalid-input"
	}
	ref, err := time.Parse(time.RFC3339, args[len(args)-1])
	if err != nil {
		return sourceReleaseInput{}, "invalid-input"
	}
	offset := 0
	if input.initial {
		offset = 1
		input.pin = args[2]
	}
	input.bundle, input.history = args[1], args[2+offset]
	input.environment, input.network, input.reference = args[3+offset], args[4+offset], ref.UTC()
	if !independentInfrastructureHistory(input.bundle, input.history) {
		return sourceReleaseInput{}, "infrastructure-paths-refused"
	}
	return input, ""
}

// openSourceRelease retains the original verifier on every successful open.
// The caller joins it after its own borrowers and must not treat a public result
// as an installation or runtime authorization.
func openSourceRelease(ctx context.Context, request sourceReleaseInput) (releaseartifact.Authorization, *release.Verifier, string, error) {
	var err error
	local := release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: request.environment, Network: request.network, RefTime: request.reference}
	var b enrollment.InfrastructureBundle
	var c enrollment.InfrastructureCandidate
	var files interface{ File(string) ([]byte, bool) }
	if request.initial {
		b, err = enrollment.VerifyInfrastructure(ctx, request.bundle, request.pin)
		files = b
	} else {
		c, err = enrollment.ReadInfrastructureCandidate(ctx, request.bundle)
		files = c
	}
	if err != nil {
		return releaseartifact.Authorization{}, nil, "infrastructure-inventory-refused", err
	}
	root, _ := files.File("root.json")
	program, _ := files.File(releaseartifact.ProgramName)
	descriptor, _ := files.File(releaseartifact.DescriptorName)
	template, _ := files.File(releaseartifact.TemplateName)
	d, err := releaseartifact.ValidateResources(descriptor, program, template)
	var f enrollment.InfrastructureFacts
	if request.initial {
		f, _ = b.Facts()
	} else {
		f, _ = c.Facts()
	}
	if err != nil || d.ReleaseIdentity != f.ReleaseIdentity || d.ReleaseVersion != f.ReleaseVersion {
		return releaseartifact.Authorization{}, nil, "infrastructure-artifact-refused", err
	}
	input := release.Inputs{RootBytes: root, Artifact: program, TargetPath: releaseartifact.ProgramTarget, Local: local, Files: map[string][]byte{}}
	for _, name := range []string{"timestamp.json", "snapshot.json", "targets.json"} {
		input.Files["https://release.invalid/metadata/"+name], _ = files.File(name)
	}
	input.Files, err = release.StaticMetadataFiles(root, input.Files["https://release.invalid/metadata/timestamp.json"], input.Files["https://release.invalid/metadata/snapshot.json"], input.Files["https://release.invalid/metadata/targets.json"])
	if err != nil {
		return releaseartifact.Authorization{}, nil, "infrastructure-metadata-refused", err
	}
	var v *release.Verifier
	if request.initial {
		v, err = release.Open(request.history)
	} else {
		v, err = release.OpenRetained(request.history)
	}
	if err != nil {
		return releaseartifact.Authorization{}, nil, "infrastructure-release-unavailable", err
	}
	var a releaseartifact.Authorization
	if request.initial {
		a, err = releaseartifact.AuthenticateInitial(ctx, v, b, input)
	} else {
		a, err = releaseartifact.AuthenticateCandidate(ctx, v, c, input)
	}
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

// runSourceRelease verifies the Source pair without acquiring installation
// inputs, starting a service or granting a reusable authorization.
func runSourceRelease(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || runtime.GOARCH != "amd64" || len(args) == 0 {
		return enrollmentReport(out, "invalid-input", 2)
	}
	if ctx.Err() != nil {
		return enrollmentReport(out, "release-canceled", 130)
	}
	request, invalid := parseSourceRelease(args)
	if invalid != "" {
		return enrollmentReport(out, invalid, 2)
	}
	a, v, refused, err := openSourceRelease(ctx, request)
	if refused != "" {
		return enrollmentReport(out, refused, 1)
	}
	program, generation := a.Targets()
	_, programOK := program.AcceptedDecision()
	_, generationOK := generation.AcceptedDecision()
	_, hasInitial := a.InitialFacts()
	if err == nil && (!programOK || !generationOK || hasInitial != request.initial) {
		err = releaseartifact.ErrAuthorization
	}
	err = errors.Join(err, v.Close(), ctx.Err())
	if err != nil {
		return enrollmentReport(out, "infrastructure-authentication-refused", 1)
	}
	if writeNetworkPreparationReceipt(ctx, out, []byte("{\"outcome\":\"authenticated-source-artifact\"}\n")) != nil || ctx.Err() != nil {
		return 1
	}
	return 0
}
