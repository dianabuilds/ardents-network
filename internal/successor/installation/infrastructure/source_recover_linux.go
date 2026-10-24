package infrastructure

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/network/state"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

type sourceInitialRecovery struct {
	ctx           context.Context
	verifier      *release.Verifier
	authorization Authorization
	input         SourceInputs
	prefix        *sourceRecoveryPrefix
	history       *sourceHistory
	manager       *systemd.SourceStopped
	published     *systemd.SourceReplacement
	identity      *sourceRuntimeIdentity
	namespace     *sourceNamespace
	writer        *sourceBirths
	selected      *sourceSelection
	binding       sourceBinding
	raw, unit     []byte
	bodies        map[string][]byte
	pending       *sourceBirth
	parent        os.FileInfo
	complete      bool
}

// RecoverSource owns a new finite caller and original-journal exclusion. It
// never recreates initial provenance, revives an old writer or starts Source.
// Unsupported or uncertain prefixes retain their original repair-required
// state. A successful continuation consumes inputs and returns stopped only.
func RecoverSource(ctx context.Context, v *release.Verifier, a Authorization, input SourceInputs, semantic state.SourceInspection, journal, distribution, history string) error {
	if ctx == nil || v == nil || input.original == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	if err := a.Check(ctx, v); err != nil {
		return err
	}
	r := input.Declaration()
	if journal == sourceReplacementJournal(r.InstallationRoot, digest(a.Descriptor())) {
		return recoverReplacementSource(ctx, v, a, input, semantic, journal, distribution, history)
	}
	for n := sourceAttemptNumber(0); n < sourceAttemptLimit; n++ {
		if journal == n.journal(r.InstallationRoot) || journal == filepath.Join(n.directory(r.InstallationRoot), "source-retirement-journal") {
			return inspectCompletedSourceRecovery(ctx, v, a, input, n, journal, distribution, history)
		}
	}
	return recoverInitialSource(ctx, v, a, input, journal, distribution, history)
}

func recoverInitialSource(ctx context.Context, v *release.Verifier, a Authorization, input SourceInputs, journal, distribution, history string) (err error) {
	if ctx == nil || v == nil || input.original == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	if _, bounded := ctx.Deadline(); !bounded {
		return ErrInput
	}
	r := input.Declaration()
	// Initial recovery has its own guard, account provenance and grammar.
	if journal != r.InstallationRoot+".source-initial-journal" {
		return ErrSourceRepairRequired
	}
	s := &sourceInitialRecovery{ctx: ctx, verifier: v, authorization: a, input: input}
	if err := input.original.begin(); err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, s.selected.close(), s.namespace.close(), s.manager.Close(), s.published.Close(), s.identity.close(), s.history.close(), s.writer.close())
		input.original.uses.Done()
		err = errors.Join(err, input.Close(), s.prefix.close())
		if err == nil && s.complete {
			if err = a.Check(ctx, v); err == nil {
				err = s.publish()
			}
		}
		if err != nil {
			err = errors.Join(ErrSourceRepairRequired, err)
		}
	}()
	if _, initial := a.InitialFacts(); initial {
		return ErrAuthorization
	}
	if err = errors.Join(sourceInstalledPlatform(ctx), input.IndependentPaths(distribution, history), a.Check(ctx, v)); err != nil {
		return err
	}
	s.prefix, err = openSourceRecoveryPrefix(ctx, journal, "initial")
	if err != nil {
		return err
	}
	// All six original directories and credentials must be complete. This
	// positive path resumes public generation writes or complete fixed-file
	// publication. Intent-only and incomplete fixed-unit births remain blocked.
	count := len(s.prefix.records)
	if !sourceInitialRecoveryPrefixSupported(s.prefix.records) {
		return ErrSourceRepairRequired
	}
	if err = s.prepare(); err != nil {
		return err
	}
	s.history, err = openSourceHistory(ctx, v, history)
	if err != nil {
		return err
	}
	admitted, err := s.history.commitment(v, s.raw)
	if err != nil {
		return err
	}
	hashes := map[string]string{}
	for path, body := range s.bodies {
		hashes[path] = digest(body)
	}
	if err = validateSourceRecoverableInitialPrefix(s.prefix.journal.raw, s.binding, hashes, admitted); err != nil {
		return err
	}
	if err = s.prefix.readOriginalCut(s.binding.Generation, digest(s.raw), s.binding.ProgramSHA256); err != nil {
		return err
	}
	if err = verifySourceControllerProgram(ctx, a.Program()); err != nil {
		return err
	}
	s.identity, err = openSourceRootIdentity(ctx)
	if err != nil {
		return err
	}
	var paths []string
	if count >= 113 {
		// Read every completed native resource before a connection may load or
		// reference this unit. Complete-looking selected bytes alone grant none.
		s.selected, err = openSourceSelection(ctx, filepath.Join(r.InstallationRoot, "binding.json"))
		if err != nil {
			return err
		}
		if !bytes.Equal(s.selected.bindingBytes(), s.raw) {
			return ErrBinding
		}
		if err = checkSourceRecordedBirths(s.selected, s.prefix, s.bodies); err != nil {
			return err
		}
		if err = s.prefix.checkCutManager(s.prefix.cut.Boundary.ManagerIdentity); err != nil {
			return err
		}
		s.published, err = systemd.OpenSourceRecoveryReplacement(ctx, s.prefix.cut.Boundary.ManagerIdentity)
		if err != nil {
			return err
		}
		facts, e := s.published.Observe()
		if e != nil {
			return e
		}
		if err = verifySourceStopped(facts.Unit, facts.Service, r, s.binding.Generation); err != nil {
			return err
		}
		paths = facts.UnitPaths
	} else {
		s.manager, err = systemd.OpenSourceStopped(ctx)
		if err != nil {
			return err
		}
		paths, err = s.manager.CheckAbsent()
		if err != nil {
			return err
		}
	}
	s.namespace, err = openSourceNamespace(ctx, paths)
	if err != nil {
		return err
	}
	if err = s.check(); err != nil {
		return err
	}
	allowed := map[string]bool{}
	for path := range s.bodies {
		allowed[path] = true
	}
	s.writer, err = openSourceRecoveryBirths(s.prefix, s.binding.Generation, digest(s.raw), allowed)
	if err != nil {
		return err
	}
	if err = s.retain(); err != nil {
		return err
	}
	if err = s.check(); err != nil {
		return err
	}
	if s.prefix.records[count-1].Phase == "file-born" {
		path := s.prefix.records[count-1].Path
		if err = s.writer.resumeSourceBornFile(s.bodies[path], s.check); err != nil {
			return err
		}
	}
	sealed := filepath.Join(r.InstallationRoot, "generations", s.binding.Generation)
	names := sourceGenerationNames()
	slices.Sort(names)
	for i, name := range names {
		ready := 68 + i*3 + 2
		if s.writer.sequence > ready {
			continue
		}
		if s.writer.sequence != 68+i*3 {
			return ErrBinding
		}
		mode := os.FileMode(0444)
		if name == ProgramName {
			mode = 0555
		}
		path := filepath.Join(sealed, name)
		if err = s.writer.write(path, s.bodies[path], mode, 0, 0, s.check); err != nil {
			return err
		}
	}
	if err = checkSourceInventory(sealed, names); err != nil {
		return err
	}
	if s.writer.sequence == 104 {
		if err = s.writer.record("seal-intent", sealed, s.writer.directories[sealed], s.binding.Generation, 0555, 0, 0); err != nil {
			return err
		}
	}
	if s.writer.sequence == 105 {
		if err = sealSourceDirectory(s.writer, sealed, s.check); err != nil {
			return err
		}
		if err = s.writer.record("sealed", sealed, s.writer.directories[sealed], s.binding.Generation, 0555, 0, 0); err != nil {
			return err
		}
	}
	selected := filepath.Join(r.InstallationRoot, "binding.json")
	if s.writer.sequence == 106 {
		if err = s.writer.write(selected, s.raw, 0644, 0, 0, s.check); err != nil {
			return err
		}
	}
	if s.writer.sequence == 109 {
		if err = s.writer.record("selected", selected, nil, digest(s.raw), 0644, 0, 0); err != nil {
			return err
		}
	}
	unitPath := "/etc/systemd/system/ardents-source.service"
	if s.writer.sequence == 110 {
		if err = s.writer.write(unitPath, s.unit, 0644, 0, 0, s.check); err != nil {
			return err
		}
	}
	if s.writer.sequence == 113 {
		if err = s.writer.record("reload-intent", unitPath, nil, digest(s.unit), 0644, 0, 0); err != nil {
			return err
		}
	}
	if err = s.check(); err != nil {
		return err
	}
	if s.writer.sequence == 114 {
		if s.published != nil {
			err = s.published.Reload()
		} else {
			err = s.manager.Reload()
		}
		if err != nil {
			return err
		}
		if err = s.namespace.reobserveAfterReload(); err != nil {
			return err
		}
	} else if s.writer.sequence != 115 {
		return ErrBinding
	}
	if s.selected == nil {
		s.selected, err = openSourceSelection(ctx, selected)
		if err != nil {
			return err
		}
	}
	if !bytes.Equal(s.selected.bindingBytes(), s.raw) {
		return ErrBinding
	}
	for round := 0; round < 2; round++ {
		u, service, e := s.observeLoaded()
		if e != nil {
			return e
		}
		if err = verifySourceStopped(u, service, r, s.binding.Generation); err != nil {
			return err
		}
		if err = errors.Join(s.selected.observe(), s.check()); err != nil {
			return err
		}
	}
	if s.writer.sequence == 114 {
		if err = s.writer.record("stopped-verified", selected, nil, digest(s.raw), 0644, 0, 0); err != nil {
			return err
		}
	}
	if err = s.writer.finishSourceRecoveryRecords(s.raw, hashes, admitted, ""); err != nil {
		return err
	}
	if err = s.verifyReadback(hashes, admitted); err != nil {
		return err
	}
	if err = s.check(); err != nil {
		return err
	}
	s.complete = true
	return nil
}

func (s *sourceInitialRecovery) prepare() error {
	p := s.input.original
	r := s.input.Declaration()
	if len(p.files) != 5 || s.authorization.origin == nil {
		return ErrInput
	}
	uid, gid := s.prefix.records[4].UID, s.prefix.records[4].GID
	generation := digest(s.authorization.Descriptor())
	unit, err := RenderSourceUnit(s.authorization.Template(), r, generation)
	if err != nil {
		return err
	}
	s.unit = unit
	files := map[string][]byte{}
	for _, name := range []string{ProgramName, TemplateName, DescriptorName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		body, ok := s.authorization.origin.File(name)
		if !ok {
			return ErrBinding
		}
		files[name] = body
	}
	files["source-plan.json"], files["source-request.json"], files["rendered-source.service"] = p.files[1].raw, p.files[0].raw, unit
	s.binding = sourceBinding{Schema: "ardents-source-binding-v1", Invocation: "source", Generation: generation, RequestSHA256: digest(p.files[0].raw), ProgramTarget: ProgramTarget, GenerationTarget: GenerationTarget,
		ProgramSHA256: digest(s.authorization.Program()), TemplateSHA256: digest(s.authorization.Template()), InventorySHA256: digest(files["infrastructure-enrollment.json"]), TargetsSHA256: digest(files["targets.json"]), PlanSHA256: digest(files["source-plan.json"]), UnitSHA256: digest(unit), Account: "ardents-source", UID: uid, GID: gid, Request: r}
	raw, err := json.Marshal(s.binding)
	if err != nil {
		return err
	}
	s.raw = append(raw, '\n')
	if _, err = decodeSourceBinding(s.raw); err != nil {
		return err
	}
	files["binding.json"] = s.raw
	s.bodies = map[string][]byte{filepath.Join(r.InstallationRoot, "binding.json"): s.raw, "/etc/systemd/system/ardents-source.service": unit, sourceInitialPendingPath(r.InstallationRoot): sourceInitialPending(digest(s.raw))}
	sealed := filepath.Join(r.InstallationRoot, "generations", generation)
	for name, body := range files {
		s.bodies[filepath.Join(sealed, name)] = body
	}
	for i, name := range []string{"certificate.pem", "private-key.pem", "client-ca.pem"} {
		s.bodies[filepath.Join(r.CredentialDirectory, name)] = p.files[i+2].raw
	}
	return checkSourceRuntimePair(p.files[2].raw, p.files[3].raw)
}

func (s *sourceInitialRecovery) check() error {
	if err := errors.Join(s.ctx.Err(), s.input.Check(), s.authorization.Check(s.ctx, s.verifier), s.history.check(s.verifier), s.prefix.check(), s.identity.check(), s.namespace.check()); err != nil {
		return err
	}
	var manager string
	var err error
	if s.published != nil {
		facts, e := s.published.Observe()
		if e != nil {
			return e
		}
		if err = verifySourceStopped(facts.Unit, facts.Service, s.binding.Request, s.binding.Generation); err != nil {
			return err
		}
		manager, err = s.published.ManagerIdentity()
	} else {
		if err = s.manager.CheckOriginal(); err != nil {
			return err
		}
		manager, err = s.manager.ManagerIdentity()
	}
	if err != nil {
		return err
	}
	if err := s.prefix.checkCutManager(manager); err != nil {
		return err
	}
	if s.selected != nil {
		if err := s.selected.observe(); err != nil {
			return err
		}
	}
	if s.writer != nil {
		if err := s.writer.check(); err != nil {
			return err
		}
	}
	if err := observeSourcePrincipal(s.ctx, s.binding.UID, s.binding.GID); err != nil {
		return err
	}
	r := s.binding.Request
	for _, path := range []string{r.StateRoot, r.ParticipationRoot} {
		entries, err := os.ReadDir(path)
		if err != nil || len(entries) != 0 {
			return errors.Join(ErrBinding, err)
		}
	}
	for n := sourceAttemptNumber(0); n < sourceAttemptLimit; n++ {
		for _, path := range []string{n.directory(r.InstallationRoot), n.journal(r.InstallationRoot)} {
			if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	others, err := filepath.Glob(r.InstallationRoot + ".source-replacement-*")
	if err != nil || len(others) != 0 {
		return errors.Join(ErrBinding, err)
	}
	return s.checkGenerationEntries()
}

func (s *sourceInitialRecovery) observeLoaded() (systemd.Properties, systemd.Properties, error) {
	if s.published != nil {
		facts, err := s.published.Observe()
		return facts.Unit, facts.Service, err
	}
	return s.manager.ObserveLoaded()
}
