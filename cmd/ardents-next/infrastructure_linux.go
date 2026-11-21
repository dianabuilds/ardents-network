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
	if args[0] == "prepare-descriptor" || args[0] == "prepare-runtime-descriptor" || args[0] == "prepare-runtime-inventory" || args[0] == "prepare-inventory" {
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
	if ctx == nil || os.Geteuid() != 0 || len(args) == 0 {
		return nil, infrastructure.ErrInput
	}
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
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return nil, infrastructure.ErrInput
		}
		s, e := openSigningInput(ctx, path, maximum, false)
		if e != nil {
			return nil, e
		}
		inputs = append(inputs, s)
		return s.raw, nil
	}
	check := func() error {
		var e error
		for _, s := range inputs {
			e = errors.Join(e, s.check(ctx))
		}
		return errors.Join(e, ctx.Err())
	}
	var destination, name string
	switch args[0] {
	case "prepare-runtime-descriptor":
		if len(args) != 7 {
			return nil, infrastructure.ErrInput
		}
		version, e := strconv.ParseInt(args[3], 10, 64)
		if e != nil || strconv.FormatInt(version, 10) != args[3] {
			return nil, infrastructure.ErrInput
		}
		program, e := read(args[4], 64<<20)
		if e != nil {
			return nil, e
		}
		template, e := read(args[5], 64<<20)
		if e != nil {
			return nil, e
		}
		result, err = infrastructure.PrepareRuntimeDescriptor(args[1], args[2], version, program, template)
		destination, name = args[6], "infrastructure-"+args[1]+".json"
	case "prepare-descriptor":
		if len(args) != 6 {
			return nil, infrastructure.ErrInput
		}
		version, e := strconv.ParseInt(args[2], 10, 64)
		if e != nil || strconv.FormatInt(version, 10) != args[2] {
			return nil, infrastructure.ErrInput
		}
		program, e := read(args[3], 64<<20)
		if e != nil {
			return nil, e
		}
		template, e := read(args[4], 64<<20)
		if e != nil {
			return nil, e
		}
		result, err = infrastructure.PrepareDescriptor(args[1], version, program, template)
		destination, name = args[5], infrastructure.DescriptorName
	case "prepare-inventory", "prepare-runtime-inventory":
		invocation := "source"
		if args[0] == "prepare-runtime-inventory" {
			if len(args) != 4 {
				return nil, infrastructure.ErrInput
			}
			invocation = args[1]
			args = []string{args[0], args[2], args[3]}
		}
		switch invocation {
		case "source", "forwarding", "issuer", "descriptor", "introduction", "join":
		default:
			return nil, infrastructure.ErrInput
		}
		if len(args) != 3 || !absoluteAdmissionPath(args[1]) {
			return nil, infrastructure.ErrInput
		}
		if args[2] == args[1] || strings.HasPrefix(args[2], args[1]+string(filepath.Separator)) {
			return nil, infrastructure.ErrInput
		}
		p, e := openPreparationParent(args[1])
		if e != nil {
			return nil, e
		}
		defer func() {
			err = errors.Join(err, p.close())
			if err != nil {
				result = nil
			}
		}()
		entries, e := p.parent().ReadDir(8)
		if e != nil && e != io.EOF {
			return nil, e
		}
		if len(entries) != 7 {
			return nil, infrastructure.ErrInput
		}
		templateName, descriptorName := "ardents-"+invocation+".service", "infrastructure-"+invocation+".json"
		wanted := map[string]int64{infrastructure.ProgramName: 64 << 20, templateName: 64 << 20, descriptorName: 16 << 10, "root.json": 1 << 20, "timestamp.json": 1 << 20, "snapshot.json": 1 << 20, "targets.json": 1 << 20}
		files := map[string][]byte{}
		for _, entry := range entries {
			limit, ok := wanted[entry.Name()]
			if !ok {
				return nil, infrastructure.ErrInput
			}
			files[entry.Name()], e = read(filepath.Join(args[1], entry.Name()), limit)
			if e != nil {
				return nil, e
			}
		}
		var d infrastructure.Declaration
		if json.Unmarshal(files[descriptorName], &d) != nil || d.Invocation != invocation {
			return nil, infrastructure.ErrBinding
		}
		// A preparation inventory cannot silently bless different descriptor resources.
		canonical, e := infrastructure.PrepareRuntimeDescriptor(invocation, d.ReleaseIdentity, d.ReleaseVersion, files[infrastructure.ProgramName], files[templateName])
		if e != nil || string(canonical) != string(files[descriptorName]) {
			return nil, infrastructure.ErrBinding
		}
		result, err = enrollment.PrepareInfrastructureInventory(enrollment.InfrastructureFacts{Platform: d.Platform, ReleaseIdentity: d.ReleaseIdentity, ReleaseVersion: d.ReleaseVersion, Invocation: d.Invocation}, files)
		destination, name = args[2], enrollment.InfrastructureInventoryName
		originalCheck := check
		check = func() error {
			if e := originalCheck(); e != nil {
				return e
			}
			if e := p.check(); e != nil {
				return e
			}
			if _, e := p.parent().Seek(0, io.SeekStart); e != nil {
				return e
			}
			current, e := p.parent().ReadDir(8)
			if (e != nil && e != io.EOF) || len(current) != 7 {
				return infrastructure.ErrBinding
			}
			for _, entry := range current {
				if _, ok := wanted[entry.Name()]; !ok {
					return infrastructure.ErrBinding
				}
			}
			return nil
		}
	default:
		return nil, infrastructure.ErrInput
	}
	if err != nil {
		return nil, err
	}
	if err = check(); err != nil {
		return nil, err
	}
	err = persistNetworkFiles(ctx, destination, []networkOutputFile{{name: name, raw: result}}, false, check, observe)
	return result, err
}
