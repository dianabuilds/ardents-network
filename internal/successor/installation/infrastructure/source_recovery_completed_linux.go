package infrastructure

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// An invocation journal is never replayed. Only a genuinely complete original
// start AND complete committed retirement can be classified stopped. Any
// archive/ACK/domain/native/Close uncertainty remains repair-required. This
// composition may reference/load the fixed manager unit and joins that original
// connection. It has no persistent recovery writes, Start, Stop, reload, unlink,
// initialize or reset effect.
func inspectCompletedSourceRecovery(ctx context.Context, v *release.Verifier, a Authorization, input SourceInputs, number sourceAttemptNumber, journal, distribution, history string) (err error) {
	if ctx == nil || v == nil || input.original == nil || number >= sourceAttemptLimit {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	if err = input.original.begin(); err != nil {
		return err
	}
	var retired *sourceRetirement
	var identity *sourceRuntimeIdentity
	var namespace *sourceNamespace
	var manager *systemd.SourceReplacement
	defer func() {
		err = errors.Join(err, namespace.close(), manager.Close(), identity.close(), retired.close())
		input.original.uses.Done()
		err = errors.Join(err, input.Close(), ctx.Err())
		if err != nil {
			err = errors.Join(ErrSourceRepairRequired, err)
		}
	}()
	if _, initial := a.InitialFacts(); initial {
		return ErrAuthorization
	}
	if err = errors.Join(sourceInstalledPlatform(ctx), input.IndependentPaths(distribution, history), input.Check(), a.Check(ctx, v), verifySourceControllerProgram(ctx, a.Program())); err != nil {
		return err
	}
	r := input.Declaration()
	if journal != number.journal(r.InstallationRoot) && journal != filepath.Join(number.directory(r.InstallationRoot), "source-retirement-journal") {
		return ErrBinding
	}
	retired, err = openSourceRetirement(ctx, v, filepath.Join(r.InstallationRoot, "binding.json"), history)
	if err != nil {
		return err
	}
	selected := retired.chain.initial.selection
	b := selected.binding
	if retired.chain.latest.number != number || retired.chain.latestSelection != selected || r != b.Request || digest(a.Descriptor()) != b.Generation || digest(a.Program()) != b.ProgramSHA256 || digest(a.Template()) != b.TemplateSHA256 {
		return ErrBinding
	}
	if len(input.original.files) != 5 || digest(input.original.files[0].raw) != b.RequestSHA256 || digest(input.original.files[1].raw) != b.PlanSHA256 {
		return ErrBinding
	}
	for i, name := range []string{"certificate.pem", "private-key.pem", "client-ca.pem"} {
		original := selected.files[filepath.Join(r.CredentialDirectory, name)]
		if original == nil || !bytes.Equal(original.raw, input.original.files[i+2].raw) {
			return ErrBinding
		}
	}
	identity, err = openSourceRootIdentity(ctx)
	if err != nil {
		return err
	}
	manager, err = systemd.OpenSourceReplacement(ctx)
	if err != nil {
		return err
	}
	facts, err := manager.Observe()
	if err != nil {
		return err
	}
	namespace, err = openSourceNamespace(ctx, facts.UnitPaths)
	if err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		if err = errors.Join(input.Check(), a.Check(ctx, v), retired.check(), identity.check(), namespace.check(), observeSourcePrincipal(ctx, b.UID, b.GID), cgroup.CheckSourceQuiescent(ctx)); err != nil {
			return err
		}
		facts, err = manager.Observe()
		if err != nil {
			return err
		}
		if err = retired.stopped(facts); err != nil {
			return err
		}
	}
	return errors.Join(input.Check(), a.Check(ctx, v), retired.check(), identity.check(), namespace.check(), cgroup.CheckSourceQuiescent(ctx))
}
