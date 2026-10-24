package infrastructure

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// This is a fresh replacement recovery lifetime. It retains its own exclusive
// original prefix, genuine cut result and complete predecessor retirement. No
// old writer, authorization, Source/State root or successful manager is revived.
func recoverReplacementSource(ctx context.Context, v *release.Verifier, a Authorization, input SourceInputs, semantic state.SourceInspection, journal, distribution, history string) (err error) {
	if ctx == nil || v == nil || input.original == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	s := &sourceReplacement{ctx: ctx, verifier: v, authorization: a, input: input, semantic: semantic}
	defer func() {
		s.terminal = errors.Join(s.terminal, err)
		err = s.close()
		if err == nil && s.verified {
			err = s.publish()
		}
		if err != nil {
			err = errors.Join(ErrSourceRepairRequired, err)
		}
	}()
	if err = input.original.begin(); err != nil {
		return err
	}
	s.usingInput = true
	if _, initial := a.InitialFacts(); initial {
		return ErrAuthorization
	}
	if err = errors.Join(sourceInstalledPlatform(ctx), input.IndependentPaths(distribution, history), a.Check(ctx, v), verifySourceControllerProgram(ctx, a.Program())); err != nil {
		return err
	}
	r := input.Declaration()
	if journal != sourceReplacementJournal(r.InstallationRoot, digest(a.Descriptor())) {
		return ErrBinding
	}
	s.recovery, err = openSourceRecoveryPrefix(ctx, journal, "replacement")
	if err != nil {
		return err
	}
	count := len(s.recovery.records)
	if !sourceReplacementRecoveryPrefixSupported(s.recovery.records) {
		return ErrSourceRepairRequired
	}
	// Detached current bytes locate an immutable predecessor generation only.
	// The original journal, complete chain and retirement are verified below;
	// this cue grants no native mutation or semantic authority.
	cue := &sourceSelection{ctx: ctx, files: map[string]*sourceFile{}, dirs: map[string]*os.File{}, identities: map[string]os.FileInfo{}}
	defer func() { err = errors.Join(err, cue.close()) }()
	raw, err := cue.read(filepath.Join(r.InstallationRoot, "binding.json"), 32<<10, 0644, 0, 0)
	if err != nil {
		return err
	}
	current, replacement, err := decodeSourceSelectedBinding(raw)
	if err != nil || current.Request.InstallationRoot != r.InstallationRoot {
		return errors.Join(ErrBinding, err)
	}
	previous := current.Generation
	if previous == digest(a.Descriptor()) {
		if replacement == nil {
			return ErrBinding
		}
		previous = current.Predecessor
	}
	old, err := openSourceArchivedSelection(ctx, r.InstallationRoot, previous)
	var originalHistory *sourceHistory
	transferred := false
	defer func() {
		if !transferred {
			err = errors.Join(err, old.close(), originalHistory.close())
		}
	}()
	if err != nil {
		return err
	}
	var result sourceRetirementResult
	found := false
	for n := sourceAttemptNumber(0); n < sourceAttemptLimit; n++ {
		directory := n.directory(r.InstallationRoot)
		path := filepath.Join(directory, "source-retirement.json")
		if path != s.recovery.records[4].Path {
			continue
		}
		if err = old.pinDirectory(directory, 0750, 0, old.binding.GID); err != nil {
			return err
		}
		body, e := old.readPinned(path, 64<<10, 0600, 0, 0)
		if e != nil || !sourceCanonicalRecord(body, &result) || result.Schema != "ardents-source-retirement-v1" || result.Attempt != uint32(n) {
			return errors.Join(ErrBinding, e)
		}
		s.candidate, err = prepareSourceReplacementConfiguration(old, result, digest(body), a, input)
		if err != nil {
			return err
		}
		found = true
		break
	}
	if !found {
		return ErrBinding
	}
	b := s.candidate.binding.Configuration
	if err = s.recovery.readOriginalCut(b.Generation, digest(s.candidate.raw), b.ProgramSHA256); err != nil {
		return err
	}
	originalHistory, err = openSourceHistory(ctx, v, history)
	if err != nil {
		return err
	}
	transferred = true
	s.retired, err = openSourceRecoveryPredecessor(v, old, originalHistory, s.recovery, s.candidate)
	if err != nil {
		return err
	}
	s.rootInfo = old.identities[r.InstallationRoot]
	s.identity, err = openSourceRootIdentity(ctx)
	if err != nil {
		return err
	}
	s.managerIdentity, err = sourceManagerIdentity()
	if err != nil {
		return err
	}
	if err = s.recovery.checkCutManager(s.recovery.cut.Boundary.ManagerIdentity); err != nil {
		return err
	}
	s.manager, err = systemd.OpenSourceRecoveryReplacement(ctx, s.recovery.cut.Boundary.ManagerIdentity)
	if err != nil {
		return err
	}
	facts, err := s.manager.Observe()
	if err != nil {
		return err
	}
	s.namespace, err = openSourceNamespace(ctx, facts.UnitPaths)
	if err != nil {
		return err
	}
	s.reloaded = count == 55
	if err = s.observe(); err != nil {
		return err
	}
	bodies := sourceReplacementRecoveryBodies(s.candidate)
	allowed := map[string]bool{}
	for path := range bodies {
		allowed[path] = true
	}
	s.births, err = openSourceRecoveryBirths(s.recovery, b.Generation, digest(s.candidate.raw), allowed)
	if err != nil {
		return err
	}
	oldInitial := s.retired.chain.initial
	oldInitial.recoveryWriter = s.births
	if err = retainSourceRecoveryBirths(s.recovery, s.births, bodies); err != nil {
		return err
	}
	for _, original := range s.births.files {
		if original.path == filepath.Join(r.InstallationRoot, "source-replacement-pending") {
			s.pending = original
		}
	}
	if s.pending == nil {
		return ErrBinding
	}
	if err = s.observe(); err != nil {
		return err
	}
	if s.recovery.records[count-1].Phase == "file-born" {
		path := s.recovery.records[count-1].Path
		if err = s.births.resumeSourceBornFile(bodies[path], s.observe); err != nil {
			return err
		}
	}
	sealed := filepath.Join(r.InstallationRoot, "generations", b.Generation)
	names := sourceGenerationNames()
	slices.Sort(names)
	for i, name := range names {
		start := 11 + i*3
		if s.births.sequence > start+2 {
			continue
		}
		if s.births.sequence != start {
			return ErrBinding
		}
		mode := os.FileMode(0444)
		if name == ProgramName {
			mode = 0555
		}
		if err = s.births.write(filepath.Join(sealed, name), s.candidate.files[name], mode, 0, 0, s.observe); err != nil {
			return err
		}
	}
	if err = checkSourceInventory(sealed, names); err != nil {
		return err
	}
	if s.births.sequence == 47 {
		if err = s.births.record("seal-intent", sealed, s.births.directories[sealed], b.Generation, 0555, 0, 0); err != nil {
			return err
		}
	}
	if s.births.sequence == 48 {
		if err = sealSourceDirectory(s.births, sealed, s.observe); err != nil {
			return err
		}
		if err = s.births.record("sealed", sealed, s.births.directories[sealed], b.Generation, 0555, 0, 0); err != nil {
			return err
		}
	}
	return s.finishReplacement()
}
