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

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	releaseartifact "github.com/dianabuilds/ardents-network/internal/successor/release/infrastructure"
)

func runInfrastructure(ctx context.Context, args []string, out io.Writer) int {
	if ctx == nil || len(args) == 0 || runtime.GOARCH != "amd64" {
		return 2
	}
	if ctx.Err() != nil {
		return 130
	}
	startSource := args[0] == "start-source"
	var sourceBinding string
	if startSource {
		if len(args) != 6 || os.Geteuid() != 0 {
			return 2
		}
		original, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		ctx = original
		sourceBinding = args[5]
		args = append(append([]string(nil), args[:5]...), time.Now().UTC().Format(time.RFC3339Nano))
		args[0] = "authenticate-candidate"
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
	request, invalid := parseSourceRelease(args)
	if invalid == "invalid-input" {
		return 2
	}
	if invalid != "" {
		return enrollmentReport(out, invalid, 2)
	}
	if (preflight || installStopped) && source.IndependentPaths(request.bundle, request.history) != nil {
		return enrollmentReport(out, "source-preflight-paths-refused", 2)
	}
	a, v, refused, err := openSourceRelease(ctx, request, false)
	if refused != "" {
		return enrollmentReport(out, refused, 1)
	}
	if err == nil && preflight {
		err = infrastructure.ObserveSource(v, a, source, request.bundle, request.history)
	}
	if err == nil && installStopped {
		err = infrastructure.InstallInitialStoppedSource(v, a, source, request.bundle, request.history)
	}
	if err == nil && startSource {
		err = executeSourceFirstStart(ctx, v, a, sourceBinding, request.history, out)
	}
	p, g := a.Targets()
	pd, pok := p.AcceptedDecision()
	gd, gok := g.AcceptedDecision()
	_, hasInitial := a.InitialFacts()
	if err == nil && (!pok || !gok || hasInitial != request.initial) {
		err = infrastructure.ErrAuthorization
	}
	err = errors.Join(err, source.Close(), v.Close(), ctx.Err())
	if err != nil {
		return enrollmentReport(out, "infrastructure-authentication-refused", 1)
	}
	if startSource {
		return enrollmentReport(out, "source-first-start-completed", 0)
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
	if writeInfrastructureReleaseReceipt(ctx, out, "authenticated-source-artifact", a, pd, gd) != nil {
		return 1
	}
	return 0
}

// A bounded failure notice never releases the original operation or its
// verifier. Even an uncertain Start continues retaining its own native joins.
func executeSourceFirstStart(ctx context.Context, v *release.Verifier, a releaseartifact.Authorization, binding, history string, out io.Writer) error {
	done := make(chan error, 1)
	go func() { done <- infrastructure.StartSource(ctx, v, a, binding, history) }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
	}
	notice := time.NewTimer(5 * time.Second)
	defer notice.Stop()
	select {
	case err := <-done:
		return err
	case <-notice.C:
		output := writeNetworkPreparationReceipt(context.WithoutCancel(ctx), out, []byte("{\"outcome\":\"source-start-completion-unavailable\"}\n"))
		return errors.Join(errors.New("source first-start completion unavailable"), output, <-done)
	}
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
		result, err = releaseartifact.PrepareDescriptor(args[1], version, program, template)
		destination, name = args[5], releaseartifact.DescriptorName
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
		wanted := map[string]int64{releaseartifact.ProgramName: 64 << 20, releaseartifact.TemplateName: 64 << 20, releaseartifact.DescriptorName: 16 << 10, "root.json": 1 << 20, "timestamp.json": 1 << 20, "snapshot.json": 1 << 20, "targets.json": 1 << 20}
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
		d, e := releaseartifact.DecodeDescriptor(files[releaseartifact.DescriptorName])
		if e != nil {
			return nil, e
		}
		// A preparation inventory cannot silently bless different descriptor resources.
		canonical, e := releaseartifact.PrepareDescriptor(d.ReleaseIdentity, d.ReleaseVersion, files[releaseartifact.ProgramName], files[releaseartifact.TemplateName])
		if e != nil || string(canonical) != string(files[releaseartifact.DescriptorName]) {
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
