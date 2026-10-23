package main

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

func runSourceInstallation(parent context.Context, args []string, out io.Writer) error {
	if parent == nil || len(args) == 0 {
		return errors.New("usage: ardents-node installation (install-initial-stopped|start-source|stop-source|restart-source|replace-source) [ARGUMENTS]")
	}
	initial := args[0] == "install-initial-stopped"
	stopping := args[0] == "stop-source"
	starting := args[0] == "start-source" || args[0] == "restart-source"
	replacing := args[0] == "replace-source"
	if (initial && len(args) != 7) || (stopping && len(args) != 3) || (starting && len(args) != 6) || (replacing && len(args) != 7) || (!initial && !stopping && !starting && !replacing) {
		return errors.New("usage: ardents-node installation install-initial-stopped DISTRIBUTION PIN HISTORY ENVIRONMENT NETWORK REQUEST | (start-source|restart-source) DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING | stop-source HISTORY BINDING | replace-source DISTRIBUTION HISTORY ENVIRONMENT NETWORK BINDING REQUEST")
	}
	if os.Geteuid() != 0 {
		return &sourceCommandFailure{infrastructure.ErrInput}
	}
	ctx, cancel := context.WithTimeout(parent, 2*time.Minute)
	defer cancel()
	return waitSourceCommand(ctx, out, "installation.source", func() error {
		err := performSourceInstallation(ctx, args, initial)
		if err != nil {
			return &sourceCommandFailure{err}
		}
		outcome := "first-start-completed"
		if initial {
			outcome = "initial-stopped-selected"
		} else if stopping {
			outcome = "source-stopped-completed"
		} else if args[0] == "restart-source" {
			outcome = "source-restart-completed"
		} else if replacing {
			outcome = "source-replacement-stopped-selected"
		}
		if err := writeSourceCommandOutcome(ctx, out, "installation.source", outcome); err != nil {
			return &sourceCommandFailure{err}
		}
		return nil
	})
}

func performSourceInstallation(ctx context.Context, args []string, initial bool) (err error) {
	var inputs infrastructure.SourceInputs
	var inspection state.SourceInspection
	replacing := args[0] == "replace-source"
	var verifier *release.Verifier
	defer func() {
		err = errors.Join(err, inspection.Close())
		err = errors.Join(err, inputs.Close())
		if verifier != nil {
			err = errors.Join(err, verifier.Close())
		}
		err = errors.Join(err, ctx.Err())
	}()
	if args[0] == "stop-source" {
		verifier, err = release.OpenRetained(args[1])
		if err != nil {
			return err
		}
		return infrastructure.StopSource(ctx, verifier, args[2], args[1])
	}
	offset := 0
	if initial {
		offset = 1
	}
	distribution, history := args[1], args[2+offset]
	if !sourceInstallationIndependent(distribution, history) {
		return infrastructure.ErrBinding
	}
	if initial || replacing {
		inputs, err = infrastructure.ReadSourceInputs(ctx, args[6])
		if err != nil {
			return err
		}
		if err = inputs.IndependentPaths(distribution, history); err != nil {
			return err
		}
	}
	var bundle enrollment.InfrastructureBundle
	var candidate enrollment.InfrastructureCandidate
	var files interface{ File(string) ([]byte, bool) }
	if initial {
		bundle, err = enrollment.VerifyInfrastructure(ctx, distribution, args[2])
		files = bundle
	} else {
		candidate, err = enrollment.ReadInfrastructureCandidate(ctx, distribution)
		files = candidate
	}
	if err != nil {
		return err
	}
	get := func(name string) []byte { raw, _ := files.File(name); return raw }
	metadata, err := release.StaticMetadataFiles(get("root.json"), get("timestamp.json"), get("snapshot.json"), get("targets.json"))
	if err != nil {
		return err
	}
	input := release.Inputs{RootBytes: get("root.json"), Artifact: get(infrastructure.ProgramName), TargetPath: infrastructure.ProgramTarget, Files: metadata,
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: args[3+offset], Network: args[4+offset], RefTime: time.Now().UTC()}}
	if initial {
		verifier, err = release.Open(history)
	} else {
		verifier, err = release.OpenRetained(history)
	}
	if err != nil {
		return err
	}
	var authorization infrastructure.Authorization
	if initial {
		authorization, err = infrastructure.AuthenticateInitial(ctx, verifier, bundle, input)
	} else {
		authorization, err = infrastructure.AuthenticateCandidate(ctx, verifier, candidate, input)
	}
	if err != nil {
		return err
	}
	if err = authorization.Check(ctx, verifier); err != nil {
		return err
	}
	if initial {
		return infrastructure.InstallInitialStoppedSource(verifier, authorization, inputs, distribution, history)
	}
	if replacing {
		raw, err := inputs.ServingPlan()
		if err != nil {
			return err
		}
		plan, err := source.DecodeServingDeclaration(raw)
		if err != nil {
			return err
		}
		config, err := installedSourceStateConfig(plan)
		if err != nil {
			return err
		}
		inspection, err = state.OpenSourceInspection(ctx, config)
		if err != nil {
			return err
		}
		return infrastructure.ReplaceSource(ctx, verifier, authorization, inputs, inspection, args[5], distribution, history)
	}
	if args[0] == "restart-source" {
		return infrastructure.RestartSource(ctx, verifier, authorization, args[5], history)
	}
	return infrastructure.StartSource(ctx, verifier, authorization, args[5], history)
}

// Check path declarations before Release can create or advance history.
// Authentication and Installation still retain their own independent native
// ancestors, immutable inputs and original verifier custody.
func sourceInstallationIndependent(distribution, history string) bool {
	for _, path := range []string{distribution, history} {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" || strings.ContainsRune(path, 0) {
			return false
		}
		for p := path; ; p = filepath.Dir(p) {
			info, err := os.Lstat(p)
			if err != nil && !os.IsNotExist(err) {
				return false
			}
			if err == nil && (!info.IsDir() || info.Mode()&os.ModeSymlink != 0) {
				return false
			}
			if p == "/" {
				break
			}
		}
	}
	inside := func(parent, child string) bool {
		relative, err := filepath.Rel(parent, child)
		return err == nil && (relative == "." || relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
	}
	return !inside(distribution, history) && !inside(history, distribution)
}
