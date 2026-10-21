//go:build linux

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func runInfrastructure(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) == 0 || runtime.GOARCH != "amd64" {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	if args[0] == "prepare-descriptor" || args[0] == "prepare-inventory" {
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
	preflight := args[0] == "preflight-initial" || args[0] == "preflight-candidate"
	installStopped := args[0] == "install-initial-stopped"
	var source infrastructure.SourceInputs
	if preflight || installStopped {
		if ((args[0] == "preflight-initial" || installStopped) && len(args) != 8) || (args[0] == "preflight-candidate" && len(args) != 7) {
			return 2
		}
		duration := 30 * time.Second
		if installStopped {
			duration = 2 * time.Minute
		}
		original, cancel := context.WithTimeout(ctx, duration)
		defer cancel()
		ctx = original
		var err error
		source, err = infrastructure.ReadSourceInputs(ctx, args[len(args)-1])
		if err != nil {
			_ = source.Close()
			return enrollmentReport(out, "source-preflight-inputs-refused", 1)
		}
		defer source.Close()
		args = append([]string(nil), args[:len(args)-1]...)
		args[0] = strings.Replace(args[0], "preflight-", "authenticate-", 1)
		if installStopped {
			args[0] = "authenticate-initial"
		}
	}
	initial := args[0] == "authenticate-initial"
	if (initial && len(args) != 7) || (!initial && (args[0] != "authenticate-candidate" || len(args) != 6)) {
		return 2
	}
	ref, err := time.Parse(time.RFC3339, args[len(args)-1])
	if err != nil {
		return 2
	}
	offset := 0
	if initial {
		offset = 1
	}
	history := args[2+offset]
	if !independentInfrastructureHistory(args[1], history) {
		return enrollmentReport(out, "infrastructure-paths-refused", 2)
	}
	if (preflight || installStopped) && source.IndependentPaths(args[1], history) != nil {
		return enrollmentReport(out, "source-preflight-paths-refused", 2)
	}
	local := release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[3+offset], Network: args[4+offset], RefTime: ref.UTC()}
	var b enrollment.InfrastructureBundle
	var c enrollment.InfrastructureCandidate
	var files interface{ File(string) ([]byte, bool) }
	if initial {
		b, err = enrollment.VerifyInfrastructure(ctx, args[1], args[2])
		files = b
	} else {
		c, err = enrollment.ReadInfrastructureCandidate(ctx, args[1])
		files = c
	}
	if err != nil {
		return enrollmentReport(out, "infrastructure-inventory-refused", 1)
	}
	root, _ := files.File("root.json")
	program, _ := files.File(infrastructure.ProgramName)
	descriptor, _ := files.File(infrastructure.DescriptorName)
	template, _ := files.File(infrastructure.TemplateName)
	d, err := infrastructure.ValidateResources(descriptor, program, template)
	var f enrollment.InfrastructureFacts
	if initial {
		f, _ = b.Facts()
	} else {
		f, _ = c.Facts()
	}
	if err != nil || d.ReleaseIdentity != f.ReleaseIdentity || d.ReleaseVersion != f.ReleaseVersion {
		return enrollmentReport(out, "infrastructure-artifact-refused", 1)
	}
	input := release.Inputs{RootBytes: root, Artifact: program, TargetPath: infrastructure.ProgramTarget, Local: local, Files: map[string][]byte{}}
	for _, name := range []string{"timestamp.json", "snapshot.json", "targets.json"} {
		input.Files["https://release.invalid/metadata/"+name], _ = files.File(name)
	}
	input.Files, err = release.StaticMetadataFiles(root, input.Files["https://release.invalid/metadata/timestamp.json"], input.Files["https://release.invalid/metadata/snapshot.json"], input.Files["https://release.invalid/metadata/targets.json"])
	if err != nil {
		return enrollmentReport(out, "infrastructure-metadata-refused", 1)
	}
	var v *release.Verifier
	if initial {
		v, err = release.Open(history)
	} else {
		v, err = release.OpenRetained(history)
	}
	if err != nil {
		return enrollmentReport(out, "infrastructure-release-unavailable", 1)
	}
	var a infrastructure.Authorization
	if initial {
		a, err = infrastructure.AuthenticateInitial(ctx, v, b, input)
	} else {
		a, err = infrastructure.AuthenticateCandidate(ctx, v, c, input)
	}
	if err == nil {
		err = a.Check(ctx, v)
	}
	if err == nil && preflight {
		err = infrastructure.ObserveSource(v, a, source, args[1], history)
	}
	if err == nil && installStopped {
		err = infrastructure.InstallInitialStoppedSource(v, a, source, args[1], history)
	}
	p, g := a.Targets()
	pd, pok := p.AcceptedDecision()
	gd, gok := g.AcceptedDecision()
	_, hasInitial := a.InitialFacts()
	if err == nil && (!pok || !gok || hasInitial != initial) {
		err = infrastructure.ErrAuthorization
	}
	err = errors.Join(err, source.Close(), v.Close(), ctx.Err())
	if err != nil {
		return enrollmentReport(out, "infrastructure-authentication-refused", 1)
	}
	if installStopped {
		if writeNetworkPreparationReceipt(ctx, out, []byte("{\"outcome\":\"source-initial-stopped-selected\"}\n")) != nil || ctx.Err() != nil {
			return 1
		}
		return 0
	}
	if preflight {
		if writeNetworkPreparationReceipt(ctx, out, []byte("{\"outcome\":\"source-read-only-preflight-observed\"}\n")) != nil || ctx.Err() != nil {
			return 1
		}
		return 0
	}
	receipt, e := json.Marshal(struct {
		Outcome           string          `json:"outcome"`
		ProgramOutcome    release.Outcome `json:"program_outcome"`
		GenerationOutcome release.Outcome `json:"generation_outcome"`
		ProgramSHA256     string          `json:"program_sha256"`
		DescriptorSHA256  string          `json:"descriptor_sha256"`
		TemplateSHA256    string          `json:"template_sha256"`
	}{"authenticated-source-artifact", pd.Outcome, gd.Outcome, fmt.Sprintf("%x", sha256.Sum256(a.Program())), fmt.Sprintf("%x", sha256.Sum256(a.Descriptor())), fmt.Sprintf("%x", sha256.Sum256(a.Template()))})
	if e != nil || writeNetworkPreparationReceipt(ctx, out, append(receipt, '\n')) != nil || ctx.Err() != nil {
		return 1
	}
	return 0
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
	case "prepare-inventory":
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
		wanted := map[string]int64{infrastructure.ProgramName: 64 << 20, infrastructure.TemplateName: 64 << 20, infrastructure.DescriptorName: 16 << 10, "root.json": 1 << 20, "timestamp.json": 1 << 20, "snapshot.json": 1 << 20, "targets.json": 1 << 20}
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
		d, e := infrastructure.DecodeDescriptor(files[infrastructure.DescriptorName])
		if e != nil {
			return nil, e
		}
		// A preparation inventory cannot silently bless different descriptor resources.
		canonical, e := infrastructure.PrepareDescriptor(d.ReleaseIdentity, d.ReleaseVersion, files[infrastructure.ProgramName], files[infrastructure.TemplateName])
		if e != nil || string(canonical) != string(files[infrastructure.DescriptorName]) {
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
