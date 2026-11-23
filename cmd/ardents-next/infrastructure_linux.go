//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/enrollment"
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
		if err != nil || writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil || ctx.Err() != nil {
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
	if e != nil || writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil || ctx.Err() != nil {
		return 1
	}
	return 0
}

func sourceReleaseEnvironment(environment, network string, ref time.Time) release.LocalEnvironment {
	return release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: environment, Network: network, RefTime: ref.UTC()}
}

// Public preparation borrows the existing original native input/output custody.
// It never writes metadata, installs a program, creates a history or starts work.
func executeInfrastructurePreparation(ctx context.Context, args []string, observe func(string, *os.File)) (result []byte, err error) {
	if ctx == nil || os.Geteuid() != 0 || len(args) < 2 {
		return nil, infrastructure.ErrInput
	}
	operation, invocation, args := args[0], args[1], args[2:]
	switch invocation {
	case "source", "forwarding", "issuer", "descriptor", "introduction", "join":
	default:
		return nil, infrastructure.ErrInput
	}
	templateName, descriptorName := "ardents-"+invocation+".service", "infrastructure-"+invocation+".json"
	var inputs []*signingInput
	defer func() {
		for i := len(inputs) - 1; i >= 0; i-- {
			err = errors.Join(err, inputs[i].close())
		}
		err = errors.Join(err, ctx.Err())
		if err != nil {
			result = nil
		}
	}()
	read := func(path string, maximum int64) ([]byte, error) {
		if !infrastructurePreparationPath(path) {
			return nil, infrastructure.ErrInput
		}
		input, err := openSigningInput(ctx, path, maximum, false)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, input)
		if err := requirePreparationExt4(input.file); err != nil {
			return nil, err
		}
		return input.raw, nil
	}
	var checkDistribution func() error
	checkIdentities := func() error {
		var err error
		if checkDistribution != nil {
			err = checkDistribution()
		}
		// A later content or directory read must not hide replacement of an
		// earlier input before the result is handed back to the caller.
		for _, input := range inputs {
			err = errors.Join(err, input.checkIdentity(ctx))
		}
		return errors.Join(err, ctx.Err())
	}
	check := func() error {
		var err error
		for _, input := range inputs {
			err = errors.Join(err, input.check(ctx))
		}
		return errors.Join(err, checkIdentities())
	}
	var destination, name string
	switch operation {
	case "prepare-runtime-descriptor":
		if len(args) != 5 || !infrastructurePreparationPath(args[4]) {
			return nil, infrastructure.ErrInput
		}
		version, parseErr := strconv.ParseInt(args[1], 10, 64)
		if parseErr != nil || strconv.FormatInt(version, 10) != args[1] {
			return nil, infrastructure.ErrInput
		}
		program, readErr := read(args[2], 64<<20)
		if readErr != nil {
			return nil, readErr
		}
		template, readErr := read(args[3], 64<<20)
		if readErr != nil {
			return nil, readErr
		}
		result, err = infrastructure.PrepareRuntimeDescriptor(invocation, args[0], version, program, template)
		if err != nil {
			return nil, err
		}
		destination, name = args[4], descriptorName
	case "prepare-runtime-inventory":
		if len(args) != 2 || !infrastructurePreparationPath(args[0]) || !infrastructurePreparationPath(args[1]) ||
			args[1] == args[0] || strings.HasPrefix(args[1], args[0]+"/") {
			return nil, infrastructure.ErrInput
		}
		distribution, openErr := openPreparationParent(args[0])
		if openErr != nil {
			return nil, openErr
		}
		defer func() {
			err = errors.Join(err, distribution.Close())
			if err != nil {
				result = nil
			}
		}()
		wanted := map[string]int64{
			infrastructure.ProgramName: 64 << 20,
			templateName:               64 << 20,
			descriptorName:             16 << 10,
			"root.json":                1 << 20,
			"timestamp.json":           1 << 20,
			"snapshot.json":            1 << 20,
			"targets.json":             1 << 20,
		}
		checkDistribution = func() error {
			if err := signingAncestors(distribution); err != nil {
				return err
			}
			if _, err := distribution.Descriptor().Seek(0, io.SeekStart); err != nil {
				return err
			}
			names, err := distribution.Descriptor().Readdirnames(8)
			if (err != nil && err != io.EOF) || len(names) != len(wanted) {
				return errors.Join(infrastructure.ErrBinding, err)
			}
			for _, name := range names {
				if _, ok := wanted[name]; !ok {
					return infrastructure.ErrBinding
				}
			}
			return nil
		}
		if err := checkDistribution(); err != nil {
			return nil, err
		}
		files := make(map[string][]byte, len(wanted))
		for name, limit := range wanted {
			files[name], err = read(filepath.Join(args[0], name), limit)
			if err != nil {
				return nil, err
			}
		}
		declaration, bindingErr := infrastructure.ValidateRuntimeResources(invocation, files[descriptorName], files[infrastructure.ProgramName], files[templateName])
		if bindingErr != nil {
			return nil, bindingErr
		}
		result, err = enrollment.PrepareInfrastructureInventory(enrollment.InfrastructureFacts{
			Platform: declaration.Platform, ReleaseIdentity: declaration.ReleaseIdentity,
			ReleaseVersion: declaration.ReleaseVersion, Invocation: declaration.Invocation,
		}, files)
		if err != nil {
			return nil, err
		}
		destination, name = args[1], enrollment.InfrastructureInventoryName
	default:
		return nil, infrastructure.ErrInput
	}
	if err = check(); err != nil {
		return nil, err
	}
	if err = persistNetworkFiles(ctx, destination, []networkOutputFile{{name: name, raw: result}}, false, check, observe); err != nil {
		return nil, err
	}
	return result, checkIdentities()
}

func infrastructurePreparationPath(path string) bool {
	return path != "/" && len(path) <= 4096 && !strings.ContainsRune(path, 0) &&
		strings.Count(path, "/") < 128 && absoluteAdmissionPath(path)
}
