package infrastructure

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

var ErrSourceRepairRequired = errors.New("source installation: retained initial prefix requires repair")

// InstallInitialStoppedSource directly consumes original private initial
// provenance, fresh coherent Release proofs, and retained Source inputs. All
// acquired native users are joined before return, including on partial failure.
// It consumes input custody before final stopped-selection publication. This
// operation never starts Source, opens State, or returns runtime authority.
func InstallInitialStoppedSource(v *release.Verifier, a Authorization, input SourceInputs, distribution, history string) (err error) {
	started := false
	defer func() {
		if started && err != nil {
			err = errors.Join(ErrSourceRepairRequired, err)
		}
	}()
	if input.original == nil {
		return ErrInput
	}
	p := input.original
	if err = p.begin(); err != nil {
		return err
	}
	ctx := p.ctx
	var births *sourceBirths
	var pending *sourceBirth
	var pendingParent os.FileInfo
	stoppedSelected := false
	defer func() {
		p.uses.Done()
		err = errors.Join(err, input.Close())
		if err == nil && stoppedSelected {
			if err = a.Check(ctx, v); err == nil {
				err = publishSourceInitialPending(ctx, births, pending, pendingParent)
			}
		}
	}()
	// Return the initial writer lease only after all later native observers and
	// helpers have physically joined. A future independent first-start reader
	// must not overlap an initial creator still releasing its original users.
	defer func() { err = errors.Join(err, births.close()) }()
	if _, ok := a.InitialFacts(); !ok {
		return ErrAuthorization
	}
	if err = errors.Join(input.IndependentPaths(distribution, history), a.Check(ctx, v)); err != nil {
		return err
	}
	originalHistory, err := openSourceHistory(ctx, v, history)
	defer func() { err = errors.Join(err, originalHistory.close()) }()
	if err != nil {
		return err
	}
	r := input.Declaration()
	generation := digest(a.Descriptor())
	unit, err := RenderSourceUnit(a.Template(), r, generation)
	if err != nil {
		return err
	}
	// Pair agreement is checked against the supplied runtime TLS key before any
	// native birth. TLS handshake trust/time remains Source/State's own policy.
	if err = checkSourceRuntimePair(p.files[2].raw, p.files[3].raw); err != nil {
		return errors.Join(ErrBinding, err)
	}
	manager, err := systemd.OpenSourceStopped(ctx)
	defer func() { err = errors.Join(err, manager.Close()) }()
	if err != nil {
		return err
	}
	var unitPaths []string
	for round := 0; round < 2; round++ {
		if unitPaths, err = manager.CheckAbsent(); err != nil {
			return err
		}
		if err = input.observeFresh(unitPaths); err != nil {
			return err
		}
		if err = errors.Join(input.Check(), a.Check(ctx, v)); err != nil {
			return err
		}
	}
	// Unlike preflight, this bounded creator requires existing direct parents.
	// No implicit parent tree can be born outside its exact journal inventory.
	if err = input.prepareInstallAncestors(); err != nil {
		return err
	}
	journalPath := r.InstallationRoot + ".source-initial-journal"
	pendingPath := sourceInitialPendingPath(r.InstallationRoot)
	for _, path := range append(r.paths(), distribution, history, p.requestPath) {
		for _, retained := range []string{journalPath, pendingPath} {
			if contains(path, retained) || contains(retained, path) {
				return ErrBinding
			}
		}
	}
	account, err := prepareSourceAccount(ctx, input)
	defer func() { err = errors.Join(err, account.close()) }()
	if err != nil {
		return err
	}
	base := filepath.Join(r.InstallationRoot, "generations")
	dir := filepath.Join(base, generation)
	files := map[string][]byte{}
	for _, name := range []string{ProgramName, TemplateName, DescriptorName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"} {
		body, ok := a.origin.File(name)
		if !ok {
			return ErrBinding
		}
		files[name] = body
	}
	files["source-plan.json"] = append([]byte(nil), p.files[1].raw...)
	files["source-request.json"] = append([]byte(nil), p.files[0].raw...)
	files["rendered-source.service"] = unit
	bound := sourceBinding{Schema: "ardents-source-binding-v1", Invocation: "source", Generation: generation, RequestSHA256: digest(p.files[0].raw), ProgramTarget: ProgramTarget, GenerationTarget: GenerationTarget, ProgramSHA256: digest(a.Program()), TemplateSHA256: digest(a.Template()), InventorySHA256: digest(files["infrastructure-enrollment.json"]), TargetsSHA256: digest(files["targets.json"]), PlanSHA256: digest(files["source-plan.json"]), UnitSHA256: digest(unit), Account: "ardents-source", UID: account.uid, GID: account.gid, Request: r}
	binding, err := json.Marshal(bound)
	if err != nil || len(binding)+1 > 32<<10 {
		return errors.Join(ErrInput, err)
	}
	binding = append(binding, '\n')
	if _, err = decodeSourceBinding(binding); err != nil {
		return err
	}
	files["binding.json"] = binding
	historyCommitment, err := originalHistory.commitment(v, binding)
	if err != nil {
		return err
	}
	allowed := map[string]bool{pendingPath: true}
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.CredentialDirectory, base, dir, filepath.Join(r.InstallationRoot, "binding.json"), "/etc/systemd/system/ardents-source.service"} {
		allowed[path] = true
	}
	for name := range files {
		allowed[filepath.Join(dir, name)] = true
	}
	for _, name := range []string{"certificate.pem", "private-key.pem", "client-ca.pem"} {
		allowed[filepath.Join(r.CredentialDirectory, name)] = true
	}
	if err = errors.Join(input.Check(), originalHistory.check(v), a.Check(ctx, v)); err != nil {
		return err
	}
	if _, err = manager.CheckAbsent(); err != nil {
		return err
	}
	if _, e := os.Lstat(journalPath); !os.IsNotExist(e) {
		return errors.Join(ErrSourceRepairRequired, e)
	}
	if _, e := os.Lstat(pendingPath); !os.IsNotExist(e) {
		return errors.Join(ErrSourceRepairRequired, e)
	}
	pendingParent, err = os.Lstat(filepath.Dir(pendingPath))
	if err != nil || !trustedDirectory(pendingParent) {
		return errors.Join(ErrBinding, err)
	}
	managerIdentity, err := manager.ManagerIdentity()
	if err != nil {
		return err
	}
	births, err = openSourceRecoverableInitialBirths(ctx, journalPath, allowed)
	started = births != nil && births.journal != nil
	if err != nil {
		return err
	}
	if err = births.bindTransition(generation, digest(binding), "initial", managerIdentity); err != nil {
		return err
	}
	guardAdmission := func() error {
		return errors.Join(input.Check(), originalHistory.check(v), a.Check(ctx, v), births.check())
	}
	if err = births.write(pendingPath, sourceInitialPending(digest(binding)), 0600, 0, 0, guardAdmission); err != nil {
		return err
	}
	for _, original := range births.files {
		if original.path == pendingPath {
			pending = original
		}
	}
	if pending == nil {
		return ErrBinding
	}
	if err = births.record("admitted", r.InstallationRoot, nil, historyCommitment, 0, account.uid, account.gid); err != nil {
		return err
	}
	var bornAccount *sourceAccount
	var reloadedNamespace map[string]os.FileInfo
	check := func() error {
		if e := originalHistory.check(v); e != nil {
			return fmt.Errorf("original Source Release history: %w", e)
		}
		if e := manager.CheckOriginal(); e != nil {
			return fmt.Errorf("original Source manager: %w", e)
		}
		if e := input.checkSourcePhase(bornAccount, reloadedNamespace); e != nil {
			return fmt.Errorf("original Source input/account: %w", e)
		}
		if e := a.Check(ctx, v); e != nil {
			return fmt.Errorf("original Source Release: %w", e)
		}
		if e := births.check(); e != nil {
			return fmt.Errorf("original Source births: %w", e)
		}
		for _, path := range []string{r.StateRoot, r.ParticipationRoot} {
			if births.directories[path] == nil {
				continue
			}
			entries, e := os.ReadDir(path)
			if e != nil || len(entries) != 0 {
				return fmt.Errorf("source semantic directory is not empty: %w", errors.Join(ErrBinding, e))
			}
		}
		if e := input.observeInstallEnvironment(); e != nil {
			return fmt.Errorf("original Source native environment: %w", e)
		}
		for _, parent := range unitPaths {
			for _, leaf := range []string{"ardents-source.service", "ardents-source.service.d", "ardents-source.socket", "ardents-source.socket.d", "ardents-.service.d", "service.d", "socket.d"} {
				path := filepath.Join(parent, leaf)
				if path == "/etc/systemd/system/ardents-source.service" {
					owned := false
					for _, b := range births.files {
						if b.path == path {
							owned = true
							break
						}
					}
					if owned {
						continue
					}
				}
				if _, e := os.Lstat(path); !os.IsNotExist(e) {
					return fmt.Errorf("source unit namespace %s: %w", path, errors.Join(ErrBinding, e))
				}
			}
		}
		if e := input.checkSourcePhase(bornAccount, reloadedNamespace); e != nil {
			return fmt.Errorf("final original Source input/account: %w", e)
		}
		if e := a.Check(ctx, v); e != nil {
			return fmt.Errorf("final original Source Release: %w", e)
		}
		return errors.Join(originalHistory.check(v), manager.CheckOriginal(), ctx.Err())
	}
	if err = check(); err != nil {
		return err
	}
	if _, err = manager.CheckAbsent(); err != nil {
		return err
	}
	if err = account.create(births, input, check, func() error {
		bornAccount = account
		return check()
	}); err != nil {
		return err
	}
	bornAccount = account
	if err = check(); err != nil {
		return err
	}
	for _, d := range []struct {
		path     string
		mode     os.FileMode
		uid, gid uint32
	}{
		{r.InstallationRoot, 0755, 0, 0}, {base, 0755, 0, 0}, {dir, 0755, 0, 0}, {r.StateRoot, 0700, account.uid, account.gid}, {r.ParticipationRoot, 0700, account.uid, account.gid}, {r.CredentialDirectory, 0755, 0, 0},
	} {
		if err = check(); err != nil {
			return err
		}
		if err = births.directory(d.path, d.mode, d.uid, d.gid, check); err != nil {
			return err
		}
		if err = check(); err != nil {
			return err
		}
	}
	for _, credential := range []struct {
		name     string
		body     []byte
		mode     os.FileMode
		uid, gid uint32
	}{
		{"certificate.pem", p.files[2].raw, 0444, 0, 0}, {"private-key.pem", p.files[3].raw, 0600, account.uid, account.gid}, {"client-ca.pem", p.files[4].raw, 0444, 0, 0},
	} {
		if err = births.write(filepath.Join(r.CredentialDirectory, credential.name), credential.body, credential.mode, credential.uid, credential.gid, check); err != nil {
			return err
		}
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		mode := os.FileMode(0444)
		if name == ProgramName {
			mode = 0555
		}
		if err = births.write(filepath.Join(dir, name), files[name], mode, 0, 0, check); err != nil {
			return err
		}
	}
	if err = checkSourceInventory(dir, names); err != nil {
		return err
	}
	if err = births.record("seal-intent", dir, births.directories[dir], generation, 0555, 0, 0); err != nil {
		return err
	}
	if err = sealSourceDirectory(births, dir, check); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = births.record("sealed", dir, births.directories[dir], generation, 0555, 0, 0); err != nil {
		return err
	}
	if _, err = manager.CheckAbsent(); err != nil {
		return err
	}
	if err = births.write(filepath.Join(r.InstallationRoot, "binding.json"), binding, 0644, 0, 0, check); err != nil {
		return err
	}
	if err = births.record("selected", filepath.Join(r.InstallationRoot, "binding.json"), nil, digest(binding), 0644, 0, 0); err != nil {
		return err
	}
	if err = births.write("/etc/systemd/system/ardents-source.service", unit, 0644, 0, 0, check); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = births.record("reload-intent", "/etc/systemd/system/ardents-source.service", nil, digest(unit), 0644, 0, 0); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = manager.Reload(); err != nil {
		return fmt.Errorf("fixed Source daemon reload: %w", err)
	}
	if err = manager.CheckOriginal(); err != nil {
		return err
	}
	if reloadedNamespace, err = input.observeReloadedNamespace(); err != nil {
		return fmt.Errorf("reloaded Source unit namespace: %w", err)
	}
	// Independently open the selected read-only configuration. Birth custody
	// and a stopped phase do not substitute for actual installed-file readback.
	selected, err := openSourceSelection(ctx, filepath.Join(r.InstallationRoot, "binding.json"))
	defer func() { err = errors.Join(err, selected.close()) }()
	if err != nil {
		return err
	}
	if selected.binding != bound {
		return ErrBinding
	}
	for round := 0; round < 2; round++ {
		u, s, e := manager.ObserveLoaded()
		if e != nil {
			return fmt.Errorf("actual Source loaded-property observation: %w", e)
		}
		if err = verifySourceStopped(u, s, r, generation); err != nil {
			return fmt.Errorf("loaded stopped Source policy: %w", err)
		}
		if err = checkSourceInventory(dir, names); err != nil {
			return fmt.Errorf("loaded Source generation inventory: %w", err)
		}
		if err = selected.observe(); err != nil {
			return err
		}
		if err = check(); err != nil {
			return err
		}
	}
	if err = births.record("stopped-verified", filepath.Join(r.InstallationRoot, "binding.json"), nil, digest(binding), 0644, 0, 0); err != nil {
		return err
	}
	initial, err := openSourceInitialTransition(ctx, v, filepath.Join(r.InstallationRoot, "binding.json"), history, births)
	defer func() { err = errors.Join(err, initial.close()) }()
	if err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = initial.check(); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	stoppedSelected = true
	return nil
}

func checkSourceInventory(dir string, names []string) error {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(names) {
		return errors.Join(ErrBinding, err)
	}
	for i, entry := range entries {
		if entry.Name() != names[i] || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
	}
	return nil
}

func sealSourceDirectory(births *sourceBirths, path string, admit func() error) (err error) {
	f, err := os.OpenFile(path, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	before, err := f.Stat()
	if err != nil || !os.SameFile(before, births.directories[path]) {
		return errors.Join(ErrBinding, err)
	}
	if err = admit(); err != nil {
		return err
	}
	if err = errors.Join(f.Chmod(0555), f.Sync(), syncSourceParent(path)); err != nil {
		return err
	}
	after, err := f.Stat()
	if err != nil || !os.SameFile(before, after) || !sourceAccess(after, 0555, 0, 0, true) {
		return errors.Join(ErrBinding, err)
	}
	births.directories[path] = after
	return nil
}
