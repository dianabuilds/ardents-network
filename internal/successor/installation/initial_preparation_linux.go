package installation

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// One operation retains all journal/root borrowers before returning its kernel
// lease. It is private: platform/protection admission and genuine command
// composition remain necessary before calling this effect owner.
type initialPreparation struct {
	lease    *installationLease
	journal  *preparationJournal
	prepared preparedInstallation
	stage    *generationStage
	created  map[string]os.FileInfo
	terminal error
}

func prepareInitialNative(ctx context.Context, request Request, authorization Authorization) (result *initialPreparation, returnedErr error) {
	if ctx == nil || request.declared == nil {
		return nil, ErrInput
	}
	facts, initial := authorization.initial.Facts()
	p, program := authorization.program.AcceptedDecision()
	g, generation := authorization.generation.AcceptedDecision()
	if !initial || !program || !generation || facts.ManifestSHA256 != request.ManifestSHA256() ||
		!g.ReferenceTime.Equal(request.ReferenceTime()) || p.Environment != facts.Environment || p.Network != facts.Network {
		return nil, ErrAuthorization
	}
	directory := filepath.Join(request.declared.InstallationRoot, "generations", digestHex(authorization.descriptor))
	if _, err := renderEndpointUnit(authorization.resources["ardents-endpoint.service"], *request.declared, directory); err != nil {
		return nil, err
	}
	if err := preflightInitialEffects(ctx, request); err != nil {
		return nil, err
	}
	lease, err := createInitialLease(ctx, request.declared.InstallationRoot)
	if err != nil {
		return nil, err
	}
	owned := &initialPreparation{lease: lease}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
			if owned.stage != nil {
				owned.stage.retainFailure(ctx, returnedErr)
			}
			if owned.journal != nil {
				owned.terminal = errors.Join(owned.terminal, owned.journal.recordFailure(ctx, returnedErr))
			}
			returnedErr = owned.close()
		}
	}()
	// The initial preflight cannot be repeated wholesale after our own root
	// birth. Reobserve the original custody and the newly held root before intent.
	if err := observeRequestCustody(ctx, request); err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	raw, err := canonicalJSON(*request.declared)
	if err != nil {
		return nil, err
	}
	record := preparationRecord{Schema: "ardents-endpoint-installation-preparation-v1", GenerationDigest: digestHex(authorization.descriptor),
		RequestDigest: digestHex(raw), Phase: "creating-account"}
	owned.journal, err = createPreparationJournal(ctx, filepath.Join(lease.path, "preparation"), record)
	if err != nil {
		return nil, err
	}
	if err := createEndpointAccount(ctx); err != nil {
		return nil, err
	}
	uid, gid, err := observeEndpointAccount()
	if err != nil {
		return nil, err
	}
	owned.prepared.uid, owned.prepared.gid = uid, gid
	record.Phase, record.UID, record.GID = "creating-mutable-roots", uid, gid
	if err := owned.journal.append(ctx, record); err != nil {
		return nil, err
	}
	paths, err := writableDirectories(*request.declared)
	if err != nil {
		return nil, err
	}
	created := make(map[string]os.FileInfo)
	owned.created = created
	for _, path := range paths {
		if err := createManagedDirectory(ctx, path, uid, gid, created); err != nil {
			return nil, err
		}
	}
	for _, path := range mutableRoots(request.declared.Headless) {
		info, err := os.Lstat(path)
		if err != nil || !managedDirectory(info, uid, gid) || !os.SameFile(created[path], info) {
			return nil, errors.Join(ErrBinding, err)
		}
		native := info.Sys().(*syscall.Stat_t)
		owned.prepared.roots = append(owned.prepared.roots, rootIdentity{Path: path, Device: uint64(native.Dev), Inode: native.Ino})
	}
	record.Phase = "mutable-roots-prepared"
	if err := owned.journal.append(ctx, record); err != nil {
		return nil, err
	}
	if err := observeRequestCustody(ctx, request); err != nil {
		return nil, err
	}
	if err := lease.observe(); err != nil {
		return nil, err
	}
	files, selected, err := assembleGeneration(ctx, request, authorization, owned.prepared)
	if err != nil {
		return nil, err
	}
	owned.stage, err = stageInitialGeneration(ctx, lease, request, files, selected, gid)
	if err != nil {
		return nil, err
	}
	if err := owned.observe(ctx, request); err != nil {
		return nil, err
	}
	if err := owned.installFixedResources(ctx, request); err != nil {
		return nil, err
	}
	if err := owned.promoteGenerationAccess(ctx, request); err != nil {
		return nil, err
	}
	if err := owned.publishInitialSelection(ctx, request); err != nil {
		return nil, err
	}
	if err := owned.archiveInitialIntent(ctx, request); err != nil {
		return nil, err
	}
	return owned, nil
}

func (owned *initialPreparation) observe(ctx context.Context, request Request) error {
	if err := observeRequestCustody(ctx, request); err != nil {
		return err
	}
	uid, gid, err := observeEndpointAccount()
	if err != nil || uid != owned.prepared.uid || gid != owned.prepared.gid {
		return errors.Join(ErrBinding, err)
	}
	for path, original := range owned.created {
		info, err := os.Lstat(path)
		if err != nil || !managedDirectory(info, uid, gid) || !os.SameFile(original, info) {
			return errors.Join(ErrBinding, err)
		}
	}
	if err := owned.lease.observe(); err != nil {
		return err
	}
	if err := errors.Join(owned.journal.observe(), owned.journal.observeInventory()); err != nil {
		return err
	}
	if err := owned.stage.observe(); err != nil {
		return err
	}
	return ctx.Err()
}

func (owned *initialPreparation) close() error {
	if owned == nil {
		return nil
	}
	if owned.stage != nil {
		owned.terminal = errors.Join(owned.terminal, owned.stage.close())
		owned.stage = nil
	}
	if owned.journal != nil {
		owned.terminal = errors.Join(owned.terminal, owned.journal.close())
		owned.journal = nil
	}
	if owned.lease != nil {
		owned.terminal = errors.Join(owned.terminal, owned.lease.close())
		owned.lease = nil
	}
	return owned.terminal
}

func createEndpointAccount(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	command := exec.CommandContext(bounded, "/usr/sbin/useradd", "--system", "--user-group", "--no-create-home", "--home-dir", "/nonexistent", "--shell", "/usr/sbin/nologin", "ardents-endpoint")
	command.Env = []string{"PATH=/usr/bin:/bin", "LANG=C", "LC_ALL=C"}
	command.WaitDelay = 5 * time.Second
	var output, diagnostic boundedManagerOutput
	command.Stdout, command.Stderr = &output, &diagnostic
	return errors.Join(command.Run(), bounded.Err(), ctx.Err())
}

func observeEndpointAccount() (uint32, uint32, error) {
	account, err := user.Lookup("ardents-endpoint")
	if err != nil {
		return 0, 0, err
	}
	uid, uidErr := strconv.ParseUint(account.Uid, 10, 32)
	gid, gidErr := strconv.ParseUint(account.Gid, 10, 32)
	group, groupErr := user.LookupGroup("ardents-endpoint")
	groups, groupsErr := account.GroupIds()
	if uidErr != nil || gidErr != nil || uid == 0 || gid == 0 || uid == 1<<32-1 || gid == 1<<32-1 ||
		groupErr != nil || group.Gid != account.Gid || groupsErr != nil || len(groups) != 1 || groups[0] != account.Gid {
		return 0, 0, ErrNativeUnavailable
	}
	return uint32(uid), uint32(gid), nil
}
