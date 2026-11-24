//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/release/infrastructure"
)

func runInfrastructure(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) == 0 || runtime.GOARCH != "amd64" {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	if args[0] == "prepare-runtime-descriptor" || args[0] == "prepare-runtime-inventory" {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		raw, err := executeInfrastructurePreparation(ctx, args, nil)
		if err != nil {
			return enrollmentReport(out, "infrastructure-preparation-refused", 1)
		}
		d := sha256.Sum256(raw)
		receipt, err := json.Marshal(struct {
			Outcome string `json:"outcome"`
			SHA256  string `json:"sha256"`
		}{"infrastructure-public-prepared", hex.EncodeToString(d[:])})
		if err != nil || artifacts.WriteReport(ctx, out, append(receipt, '\n')) != nil || ctx.Err() != nil {
			return 1
		}
		return 0
	}
	initial := args[0] == "authenticate-initial"
	if (initial && len(args) != 7) || (!initial && (args[0] != "authenticate-candidate" || len(args) != 6)) {
		return 2
	}
	ref, err := time.Parse(time.RFC3339, args[len(args)-1])
	if err != nil {
		return 2
	}

	var result infrastructure.SourceReleaseInspection
	if initial {
		local := sourceReleaseEnvironment(args[4], args[5], ref)
		result, err = infrastructure.InspectInitialSourceRelease(ctx, local, args[1], args[2], args[3])
	} else {
		local := sourceReleaseEnvironment(args[3], args[4], ref)
		result, err = infrastructure.InspectCandidateSourceRelease(ctx, local, args[1], args[2])
	}
	switch {
	case errors.Is(err, infrastructure.ErrSourceReleasePaths):
		return enrollmentReport(out, "infrastructure-paths-refused", 2)
	case errors.Is(err, infrastructure.ErrSourceReleaseInventory):
		return enrollmentReport(out, "infrastructure-inventory-refused", 1)
	case errors.Is(err, infrastructure.ErrSourceReleaseArtifact):
		return enrollmentReport(out, "infrastructure-artifact-refused", 1)
	case errors.Is(err, infrastructure.ErrSourceReleaseMetadata):
		return enrollmentReport(out, "infrastructure-metadata-refused", 1)
	case errors.Is(err, infrastructure.ErrSourceReleaseHistory):
		return enrollmentReport(out, "infrastructure-release-unavailable", 1)
	case err != nil:
		return enrollmentReport(out, "infrastructure-authentication-refused", 1)
	}
	receipt, e := json.Marshal(struct {
		Outcome           string          `json:"outcome"`
		ProgramOutcome    release.Outcome `json:"program_outcome"`
		GenerationOutcome release.Outcome `json:"generation_outcome"`
		ProgramSHA256     string          `json:"program_sha256"`
		DescriptorSHA256  string          `json:"descriptor_sha256"`
		TemplateSHA256    string          `json:"template_sha256"`
	}{
		Outcome:           "authenticated-source-artifact",
		ProgramOutcome:    result.ProgramOutcome,
		GenerationOutcome: result.GenerationOutcome,
		ProgramSHA256:     hex.EncodeToString(result.ProgramSHA256[:]),
		DescriptorSHA256:  hex.EncodeToString(result.DescriptorSHA256[:]),
		TemplateSHA256:    hex.EncodeToString(result.TemplateSHA256[:]),
	})
	if e != nil || artifacts.WriteReport(ctx, out, append(receipt, '\n')) != nil || ctx.Err() != nil {
		return 1
	}
	return 0
}

func sourceReleaseEnvironment(environment, network string, ref time.Time) release.LocalEnvironment {
	return release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: environment, Network: network, RefTime: ref.UTC()}
}

func executeInfrastructurePreparation(ctx context.Context, args []string, observe func(string, *os.File)) ([]byte, error) {
	if len(args) < 2 {
		return nil, infrastructure.ErrInput
	}
	operation, role, values := args[0], args[1], args[2:]
	switch operation {
	case "prepare-runtime-descriptor":
		if len(values) != 5 {
			return nil, infrastructure.ErrInput
		}
		version, err := strconv.ParseInt(values[1], 10, 64)
		if err != nil || strconv.FormatInt(version, 10) != values[1] {
			return nil, infrastructure.ErrInput
		}
		return infrastructure.PrepareRuntimeDescriptorFile(ctx, role, values[0], version, values[2], values[3], values[4], observe)
	case "prepare-runtime-inventory":
		if len(values) != 2 {
			return nil, infrastructure.ErrInput
		}
		return infrastructure.PrepareRuntimeInventoryFile(ctx, role, values[0], values[1], observe)
	default:
		return nil, infrastructure.ErrInput
	}
}
