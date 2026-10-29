package forwarding

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"syscall"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

var ErrRepairRequired = errors.New("forwarding installation: retained initial prefix requires repair")

// InstallInitialStopped consumes the original input AND verifier lifetimes.
// They close, together with all native users, before the final pending unlink.
// It performs no Start, semantic initialization, adoption or recovery. A partial
// or uncertain birth retains the exclusive journal and blocking pending prefix.
func InstallInitialStopped(v *release.Verifier, a infrastructure.ForwardingAuthorization, input Inputs, distribution, history string) (returned error) {
	p := input.original
	if p == nil || v == nil || os.Geteuid() != 0 {
		return ErrInput
	}
	if _, finite := p.ctx.Deadline(); !finite {
		return ErrInput
	}
	if err := p.begin(); err != nil {
		return err
	}
	p.mu.Lock()
	if p.installing {
		p.mu.Unlock()
		p.uses.Done()
		return ErrInput
	}
	p.installing = true
	p.mu.Unlock()
	var births *initialBirths
	var account *nodeAccount
	var manager *systemd.ForwardingStopped
	var identity *runtimeIdentity
	var namespace *startNamespace
	var pending *initialFile
	var pendingParent os.FileInfo
	complete := false
	ctx := p.ctx
	defer func() {
		returned = errors.Join(returned, manager.Close(), namespace.close(), identity.close(), account.close(), births.close())
		p.uses.Done()
		returned = errors.Join(returned, input.Close())
		if returned == nil && complete {
			returned = errors.Join(a.Check(ctx, v), v.CheckHistoryPath(ctx, history), ctx.Err())
		}
		returned = errors.Join(returned, v.Close())
		returned = errors.Join(returned, finishTransition(ctx))
		if returned == nil && complete {
			returned = publishInitialPending(births, pending, pendingParent)
		}
		if births != nil && births.journal != nil && returned != nil {
			returned = errors.Join(ErrRepairRequired, returned)
		}
	}()
	if len(p.files) != 4 {
		return ErrInput
	}
	if _, ok := a.InitialFacts(); !ok {
		return infrastructure.ErrAuthorization
	}
	if err := errors.Join(input.IndependentPaths(distribution, history), a.Check(ctx, v), v.CheckHistoryPath(ctx, history)); err != nil {
		return err
	}
	r := p.request
	manager, returned = systemd.OpenForwardingStopped(ctx)
	if returned != nil {
		return returned
	}
	paths, err := manager.CheckAbsent()
	if err != nil {
		return err
	}
	identity, err = openRootIdentity(ctx)
	if err != nil {
		return err
	}
	namespace, err = openStartNamespace(ctx, paths)
	if err != nil {
		return err
	}
	managerIdentity, err := manager.ManagerIdentity()
	if err != nil {
		return err
	}
	// Pin only original native ancestors here. Host account postimages have
	// their own separate phase owner; they never become immutable input files.
	p.ioMu.Lock()
	for _, path := range append([]string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory}, filepath.Join(history, "observation")) {
		if _, err = p.parents(path); err != nil {
			break
		}
	}
	p.ioMu.Unlock()
	if err != nil {
		return err
	}
	historyPin := p.byPath[history]
	if historyPin == nil || !nodeAccess(historyPin.info, 0700, 0, 0, true) {
		return ErrBinding
	}
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory} {
		var storage syscall.Statfs_t
		if err := syscall.Statfs(filepath.Dir(path), &storage); err != nil || storage.Type != 0xef53 || storage.Flags&1 != 0 {
			return errors.Join(ErrBinding, err)
		}
		for parent := filepath.Dir(path); parent != "/"; parent = filepath.Dir(parent) {
			info, err := os.Lstat(parent)
			if err != nil || !trustedInputDirectory(info) || info.Mode().Perm()&0001 == 0 {
				return errors.Join(ErrBinding, err)
			}
		}
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	account, err = prepareNodeAccount(ctx, input)
	if err != nil {
		return err
	}
	generation := digest(a.Descriptor())
	unit, err := RenderUnit(a.Template(), r, generation)
	if err != nil {
		return err
	}
	files := a.Artifacts()
	if len(files) != 8 {
		return ErrBinding
	}
	files["forwarding-request.json"] = bytes.Clone(p.files[0].raw)
	files["forwarding-plan.json"] = bytes.Clone(p.files[1].raw)
	files["rendered-forwarding.service"] = unit
	b := binding{Schema: "ardents-forwarding-binding-v1", Invocation: "forwarding", Generation: generation, ProgramTarget: infrastructure.ProgramTarget, GenerationTarget: infrastructure.ForwardingGenerationTarget, ProgramSHA256: digest(a.Program()), TemplateSHA256: digest(a.Template()), InventorySHA256: digest(files["infrastructure-enrollment.json"]), TargetsSHA256: digest(files["targets.json"]), RequestSHA256: digest(p.files[0].raw), PlanSHA256: digest(p.files[1].raw), UnitSHA256: digest(unit), CertificateSHA256: digest(p.files[2].raw), Account: "ardents-node", UID: account.uid, GID: account.gid, History: history, Request: r}
	bound, err := json.Marshal(b)
	if err != nil {
		return err
	}
	bound = append(bound, '\n')
	if _, err = decodeBinding(bound); err != nil {
		return err
	}
	files["binding.json"] = bound
	base := filepath.Join(r.InstallationRoot, "generations")
	dir := filepath.Join(base, generation)
	journalPath := r.InstallationRoot + ".forwarding-initial-journal"
	pendingPath := r.InstallationRoot + ".forwarding-initial-pending"
	for _, derived := range r.reservedSiblingPaths() {
		for _, path := range append(r.paths(), distribution, history, p.path) {
			if contains(derived, path) || contains(path, derived) {
				return ErrBinding
			}
		}
		if _, err := os.Lstat(derived); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrRepairRequired, err)
		}
	}
	pendingParent, err = os.Lstat(filepath.Dir(pendingPath))
	if err != nil {
		return err
	}
	allowed := map[string]bool{"ardents-node": true, journalPath: true, pendingPath: true, history: true}
	for _, path := range []string{r.InstallationRoot, r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot, r.CredentialDirectory, base, dir, filepath.Join(r.InstallationRoot, "binding.json"), "/etc/systemd/system/ardents-forwarding.service"} {
		allowed[path] = true
	}
	for name := range files {
		allowed[filepath.Join(dir, name)] = true
	}
	for _, name := range []string{"certificate.pem", "private-key.pem"} {
		allowed[filepath.Join(r.CredentialDirectory, name)] = true
	}
	for _, path := range append([]string{"/etc/passwd", "/etc/group", "/etc/shadow", "/etc/gshadow"}, nodeAccountAuxiliaryPaths()...) {
		allowed[path] = true
	}
	if err = errors.Join(input.Check(), a.Check(ctx, v), v.CheckHistoryPath(ctx, history)); err != nil {
		return err
	}
	if _, err = manager.CheckAbsent(); err != nil {
		return err
	}
	if err = initialNamespace(paths, nil); err != nil {
		return err
	}
	nativeContext, err := captureNativeContext(identity, managerIdentity, namespace, p, nil)
	if err != nil {
		return err
	}
	if err := errors.Join(input.Check(), a.Check(ctx, v), v.CheckHistoryPath(ctx, history), identity.check(), namespace.check()); err != nil {
		return err
	}
	if err := bindTransition(ctx, "initial", journalPath, generation, bound, nativeContext, managerIdentity); err != nil {
		return err
	}
	births, err = openForwardingBirths(ctx, journalPath, allowed, "forwarding-initial-v2")
	if err != nil {
		return err
	}
	if err := births.record("native-context", r.InstallationRoot, nil, digest(nativeContext), 0, 0, 0); err != nil {
		return err
	}
	accountCreated := false
	check := func() error {
		if err := errors.Join(input.Check(), a.Check(ctx, v), v.CheckHistoryPath(ctx, history), manager.CheckOriginal(), identity.check(), namespace.check(), births.check()); err != nil {
			return err
		}
		actual, err := manager.ManagerIdentity()
		if err != nil || actual != managerIdentity {
			return errors.Join(ErrBinding, err)
		}
		if accountCreated {
			if err := account.check(); err != nil {
				return err
			}
		} else {
			if err := account.checkBefore(); err != nil {
				return err
			}
		}
		if err := initialNamespace(paths, births); err != nil {
			return err
		}
		for _, path := range []string{r.StateRoot, r.ParticipationRoot, r.SpendRoot, r.HostingRoot} {
			if births.dirs[path] != nil {
				entries, err := os.ReadDir(path)
				if err != nil || len(entries) != 0 {
					return errors.Join(ErrBinding, err)
				}
			}
		}
		return ctx.Err()
	}
	if err = births.write(pendingPath, []byte("ardents-forwarding-initial-pending-v1\n"+digest(bound)+"\n"), 0600, 0, 0, check); err != nil {
		return err
	}
	pending = births.files[len(births.files)-1]
	if err = births.record("admitted", history, historyPin.info, digest(bound), 0700, 0, 0); err != nil {
		return err
	}
	if err = account.create(births, input, check, func() error { accountCreated = true; return check() }); err != nil {
		return err
	}
	accountCreated = true
	if err = observeNodePrincipal(ctx, account.uid, account.gid); err != nil {
		return err
	}
	for _, d := range []struct {
		path     string
		mode     os.FileMode
		uid, gid uint32
	}{
		{r.InstallationRoot, 0755, 0, 0}, {base, 0755, 0, 0}, {dir, 0755, 0, 0}, {r.StateRoot, 0700, account.uid, account.gid}, {r.ParticipationRoot, 0700, account.uid, account.gid}, {r.SpendRoot, 0700, account.uid, account.gid}, {r.HostingRoot, 0700, account.uid, account.gid}, {r.CredentialDirectory, 0755, 0, 0},
	} {
		if err = births.directory(d.path, d.mode, d.uid, d.gid, check); err != nil {
			return err
		}
	}
	if err = births.write(filepath.Join(r.CredentialDirectory, "certificate.pem"), p.files[2].raw, 0444, 0, 0, check); err != nil {
		return err
	}
	if err = births.write(filepath.Join(r.CredentialDirectory, "private-key.pem"), p.files[3].raw, 0640, 0, account.gid, check); err != nil {
		return err
	}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		mode := os.FileMode(0444)
		if name == programName {
			mode = 0555
		}
		if err = births.write(filepath.Join(dir, name), files[name], mode, 0, 0, check); err != nil {
			return err
		}
	}
	if err = verifyInitialInventory(dir, files); err != nil {
		return err
	}
	if err = births.record("seal-intent", dir, births.identities[dir], generation, 0555, 0, 0); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = errors.Join(births.dirs[dir].Chmod(0555), births.dirs[dir].Sync()); err != nil {
		return err
	}
	births.identities[dir], err = births.dirs[dir].Stat()
	if err != nil {
		return err
	}
	if err = births.record("sealed", dir, births.identities[dir], generation, 0555, 0, 0); err != nil {
		return err
	}
	if err = births.write(filepath.Join(r.InstallationRoot, "binding.json"), bound, 0644, 0, 0, check); err != nil {
		return err
	}
	if err = births.record("selected", filepath.Join(r.InstallationRoot, "binding.json"), nil, digest(bound), 0644, 0, 0); err != nil {
		return err
	}
	if err = births.write("/etc/systemd/system/ardents-forwarding.service", unit, 0644, 0, 0, check); err != nil {
		return err
	}
	if err = births.record("reload-intent", "/etc/systemd/system/ardents-forwarding.service", nil, digest(unit), 0644, 0, 0); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = manager.Reload(); err != nil {
		return err
	}
	if err := namespace.reobserveAfterReload(); err != nil {
		return err
	}
	for round := 0; round < 2; round++ {
		u, s, err := manager.ObserveLoaded()
		if err != nil {
			return err
		}
		if err = verifyForwardingStopped(u, s, r, generation); err != nil {
			return err
		}
		if err = verifyInitialInventory(dir, files); err != nil {
			return err
		}
		if err = check(); err != nil {
			return err
		}
	}
	if err := observeTransitionReload(ctx, namespace); err != nil {
		return err
	}
	if err := check(); err != nil {
		return err
	}
	if err = births.record("stopped-verified", filepath.Join(r.InstallationRoot, "binding.json"), nil, digest(bound), 0644, 0, 0); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = births.record("completed", journalPath, nil, digest(bound), 0600, 0, 0); err != nil {
		return err
	}
	hashes := make(map[string]string, len(files)+5)
	for name, raw := range files {
		hashes[filepath.Join(dir, name)] = digest(raw)
	}
	hashes[filepath.Join(r.CredentialDirectory, "certificate.pem")] = digest(p.files[2].raw)
	hashes[filepath.Join(r.CredentialDirectory, "private-key.pem")] = digest(p.files[3].raw)
	hashes[filepath.Join(r.InstallationRoot, "binding.json")] = digest(bound)
	hashes["/etc/systemd/system/ardents-forwarding.service"] = digest(unit)
	hashes[pendingPath] = digest([]byte("ardents-forwarding-initial-pending-v1\n" + digest(bound) + "\n"))
	if err = verifyInitialBirthReadback(ctx, journalPath, b, hashes); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	complete = true
	return nil
}

func initialNamespace(paths []string, births *initialBirths) error {
	for _, parent := range paths {
		if !validPath(parent) {
			return ErrBinding
		}
		for _, role := range []string{"forwarding", "issuer", "descriptor", "introduction", "join"} {
			for _, suffix := range []string{".service", ".service.d", ".socket", ".socket.d"} {
				path := filepath.Join(parent, "ardents-"+role+suffix)
				owned := false
				if births != nil {
					for _, file := range births.files {
						owned = owned || file.path == path
					}
				}
				if owned {
					continue
				}
				if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
					return errors.Join(ErrBinding, err)
				}
			}
		}
		for _, leaf := range []string{"ardents-.service.d", "service.d", "socket.d"} {
			if _, err := os.Lstat(filepath.Join(parent, leaf)); !errors.Is(err, os.ErrNotExist) {
				return errors.Join(ErrBinding, err)
			}
		}
	}
	return nil
}

func verifyInitialInventory(dir string, files map[string][]byte) (returned error) {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != len(files) {
		return errors.Join(ErrBinding, err)
	}
	for _, entry := range entries {
		body, ok := files[entry.Name()]
		if !ok || entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
			return ErrBinding
		}
		path := filepath.Join(dir, entry.Name())
		f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
		if err != nil {
			return err
		}
		info, err := f.Stat()
		mode := os.FileMode(0444)
		if entry.Name() == programName {
			mode = 0555
		}
		if err == nil && !nodeAccess(info, mode, 0, 0, false) {
			err = ErrBinding
		}
		var raw []byte
		if err == nil {
			raw, err = io.ReadAll(io.LimitReader(f, int64(len(body))+1))
		}
		if err == nil && !bytes.Equal(raw, body) {
			err = ErrBinding
		}
		after, afterErr := f.Stat()
		named, nameErr := os.Lstat(path)
		if !sameInputFile(info, after) || !sameInputFile(info, named) {
			err = errors.Join(err, ErrBinding)
		}
		err = errors.Join(err, afterErr, nameErr, f.Close())
		if err != nil {
			return err
		}
	}
	return nil
}

// This is the final effect, after all original users, inputs, journal lease and
// Release verifier have physically closed. A lost unlink restores denial; there
// is no required sync/Close/observation after it that could fail after admission.
func publishInitialPending(births *initialBirths, pending *initialFile, parent os.FileInfo) error {
	if births == nil || !births.closed || births.terminal != nil || pending == nil || parent == nil {
		return ErrRepairRequired
	}
	if err := births.ctx.Err(); err != nil {
		return err
	}
	currentParent, err := os.Lstat(filepath.Dir(pending.path))
	if err != nil || !trustedInputDirectory(currentParent) || !os.SameFile(parent, currentParent) || parent.Mode() != currentParent.Mode() {
		return errors.Join(ErrBinding, err)
	}
	named, err := os.Lstat(pending.path)
	if err != nil || !sameInputFile(pending.info, named) {
		return errors.Join(ErrBinding, err)
	}
	if err := births.ctx.Err(); err != nil {
		return err
	}
	return os.Remove(pending.path)
}
