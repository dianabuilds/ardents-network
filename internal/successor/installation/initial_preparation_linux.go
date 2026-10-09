package installation

import (
	"context"
	"errors"
	manageddirectory "github.com/dianabuilds/ardents-network/internal/successor/installation/directory"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	requestinput "github.com/dianabuilds/ardents-network/internal/successor/installation/request"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

// One operation retains all journal/root borrowers before returning its kernel
// lease. It is private: platform/protection admission and genuine command
// composition remain necessary before calling this effect owner.
type initialPreparation struct {
	lease    *installationLease
	journal  *journal.Owner
	prepared preparedInstallation
	stage    *installationTransaction
	created  *manageddirectory.Creation
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
				owned.terminal = errors.Join(owned.terminal, owned.journal.RecordFailure(ctx, returnedErr))
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
	record := journal.Record{Schema: "ardents-endpoint-installation-preparation-v1", GenerationDigest: digestHex(authorization.descriptor),
		RequestDigest: digestHex(raw), Phase: "creating-account"}
	owned.journal, err = journal.Create(ctx, filepath.Join(lease.path, "preparation"), record)
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
	if err := owned.journal.Append(ctx, record); err != nil {
		return nil, err
	}
	paths, err := writableDirectories(*request.declared)
	if err != nil {
		return nil, err
	}
	owned.created, err = manageddirectory.New(ctx, uid, gid)
	if err != nil {
		return nil, directoryObservationError(err)
	}
	for _, path := range paths {
		if err := owned.created.Create(path); err != nil {
			return nil, directoryObservationError(err)
		}
	}
	for _, path := range mutableRoots(request.declared.Headless) {
		identity, err := owned.created.Identity(path)
		if err != nil {
			return nil, directoryObservationError(err)
		}
		owned.prepared.roots = append(owned.prepared.roots, rootIdentity{Path: path, Device: identity.Device, Inode: identity.Inode})
	}
	record.Phase = "mutable-roots-prepared"
	if err := owned.journal.Append(ctx, record); err != nil {
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

// Service traversal does not release the writer or grant installed startup.
// The admitted operation supplies its actual NSS-observed group and rechecks
// original roots and journals before and after both directory changes.
func (owned *initialPreparation) promoteGenerationAccess(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := owned.stage.promoteAccess(ctx, owned.prepared.gid); err != nil {
		return err
	}
	return owned.observe(ctx, request)
}

func (owned *initialPreparation) archiveInitialIntent(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := owned.observeStoppedInstallation(ctx, request); err != nil {
		return err
	}
	if err := owned.stage.archiveIntent(ctx); err != nil {
		return err
	}
	return owned.observe(ctx, request)
}

func (owned *initialPreparation) observe(ctx context.Context, request Request) error {
	if err := observeRequestCustody(ctx, request); err != nil {
		return err
	}
	uid, gid, err := observeEndpointAccount()
	if err != nil || uid != owned.prepared.uid || gid != owned.prepared.gid {
		return errors.Join(ErrBinding, err)
	}
	if err := owned.created.Observe(); err != nil {
		return directoryObservationError(err)
	}
	if err := owned.lease.observe(); err != nil {
		return err
	}
	if err := owned.journal.Observe(); err != nil {
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
		owned.terminal = errors.Join(owned.terminal, owned.journal.Close())
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
	var output, diagnostic boundedAccountOutput
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

func (owned *initialPreparation) observeStoppedInstallation(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := observeStoppedManager(ctx); err != nil {
		return err
	}
	version, err := systemd.Version(ctx)
	if err != nil {
		return err
	}
	unit, service, err := systemd.Endpoint(ctx)
	if err != nil {
		return err
	}
	if err := verifyStoppedEndpointProperties(version, unit, service, *request.declared, owned.stage.selected.GenerationDigest); err != nil {
		return err
	}
	// Unit and Service are separate manager observations. Reobserve stopped
	// identity after reading both; no active/queued process gets a late handoff.
	if err := observeStoppedManager(ctx); err != nil {
		return err
	}
	return owned.observe(ctx, request)
}

type boundedAccountOutput struct{ body strings.Builder }

func (output *boundedAccountOutput) Write(body []byte) (int, error) {
	if output.body.Len()+len(body) > 64<<10 {
		return 0, ErrNativeUnavailable
	}
	return output.body.Write(body)
}

// Preflight is an observation inside native preparation, not a reusable proof.
// Platform/protection admission must precede it in the actual provision owner.
// Repeat these observations under that operation's lease before native effects.
func preflightInitialEffects(ctx context.Context, request Request) error {
	if ctx == nil || request.declared == nil || request.ManifestSHA256() == "" {
		return ErrInput
	}
	if err := observeRequestCustody(ctx, request); err != nil {
		return err
	}
	_, err := user.Lookup("ardents-endpoint")
	var missingUser user.UnknownUserError
	if err == nil || !errors.As(err, &missingUser) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	_, err = user.LookupGroup("ardents-endpoint")
	var missingGroup user.UnknownGroupError
	if err == nil || !errors.As(err, &missingGroup) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	paths, err := writableDirectories(*request.declared)
	if err != nil {
		return err
	}
	paths = append(paths, request.declared.InstallationRoot, "/usr/lib/ardents/text-worker-root",
		"/etc/ardents/text-worker-artifact.json", "/run/ardents-text", request.declared.Headless.ApplicationSocket)
	if request.declared.Headless.Role == "" {
		paths = append(paths, request.declared.Headless.AdministrationSocket)
	}
	for filename := range fixedResourceNames() {
		paths = append(paths, filename)
	}
	for _, filename := range paths {
		if err := requireAbsentManagedPath(filename); err != nil {
			return err
		}
	}
	if err := observeAbsentInstallationUnits(ctx); err != nil {
		return err
	}
	return observeRequestCustody(ctx, request)
}

func observeRequestCustody(ctx context.Context, input Request) error {
	if ctx == nil || input.custody == nil || input.declared == nil {
		return ErrInput
	}
	err := input.custody.Observe(ctx, *input.declared)
	if errors.Is(err, requestinput.ErrChanged) {
		return ErrBinding
	}
	return err
}

func requireAbsentManagedPath(filename string) error {
	if !canonicalPath(filename) || filename == "/" {
		return ErrInput
	}
	if _, err := os.Lstat(filename); !os.IsNotExist(err) {
		return errors.Join(ErrNativeUnavailable, err)
	}
	for parent := filepath.Dir(filename); ; parent = filepath.Dir(parent) {
		_, err := os.Lstat(parent)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		_, err = rootDirectoryAncestors(parent)
		return err
	}
}

func observeAbsentInstallationUnits(ctx context.Context) error {
	body, err := systemd.Units(ctx)
	if err != nil || strings.TrimSpace(body) != "" {
		return errors.Join(ErrNativeUnavailable, err)
	}
	return ctx.Err()
}

// Initial fixed publication retains stopped manager observations and the
// original preparation. This private operation grants no platform admission.
func (owned *initialPreparation) installFixedResources(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := observeAbsentInstallationUnits(ctx); err != nil {
		return err
	}
	for filename := range fixedResourceNames() {
		if err := requireAbsentManagedPath(filename); err != nil {
			return err
		}
	}
	for _, filename := range []string{"/usr/lib/ardents/text-worker-root", "/etc/ardents/text-worker-artifact.json", "/run/ardents-text"} {
		if err := requireAbsentManagedPath(filename); err != nil {
			return err
		}
	}
	stage := owned.stage
	if err := stage.fixedPhase(ctx, "0003.json", "installing-fixed-resources"); err != nil {
		return err
	}
	workerRoot := "/usr/lib/ardents/text-worker-root"
	if err := stage.birthFixedDirectory(ctx, workerRoot, 0700, 0); err != nil {
		return err
	}
	for _, name := range []string{"dev", "proc", "sys", "run", "tmp", "etc", "root", "usr", "var", "var/tmp"} {
		if err := stage.birthFixedDirectory(ctx, filepath.Join(workerRoot, name), 0555, 0); err != nil {
			return err
		}
	}
	resources := fixedResourceNames()
	paths := make([]string, 0, len(resources))
	for filename := range resources {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	digests := make(map[string]string)
	for _, filename := range paths {
		name := resources[filename]
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		body := stage.generation.Bytes(name)
		if len(body) == 0 {
			return ErrBinding
		}
		if err := stage.ensureRootParent(ctx, filepath.Dir(filename)); err != nil {
			return err
		}
		mode := os.FileMode(0644)
		if filename == filepath.Join(workerRoot, "ardents-text") {
			mode = 0555
		}
		if err := stage.createFixedFile(ctx, filename, body, mode, 0); err != nil {
			return err
		}
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			digests[filename] = digestHex(body)
		}
	}
	if err := stage.changeFixedDirectoryMode(ctx, workerRoot, 0555); err != nil {
		return err
	}
	manifest, err := fixedArtifactManifest(digests)
	if err != nil {
		return err
	}
	if err := stage.ensureRootParent(ctx, "/etc/ardents"); err != nil {
		return err
	}
	if err := stage.createFixedFile(ctx, "/etc/ardents/text-worker-artifact.json", manifest, 0644, 0); err != nil {
		return err
	}
	if err := stage.birthFixedDirectory(ctx, "/run/ardents-text", 0710, owned.prepared.gid); err != nil {
		return err
	}
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	return stage.fixedPhase(ctx, "0004.json", "fixed-resources-installed")
}

func (owned *initialPreparation) publishInitialSelection(ctx context.Context, request Request) error {
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	if err := observeAbsentInstallationUnits(ctx); err != nil {
		return err
	}
	stage := owned.stage
	if err := stage.fixedPhase(ctx, "0005.json", "publishing-selection"); err != nil {
		return err
	}
	body, err := canonicalJSON(stage.selected)
	if err != nil {
		return err
	}
	if err := stage.createFixedFile(ctx, filepath.Join(stage.lease.path, "selection.json"), body, 0640, owned.prepared.gid); err != nil {
		return err
	}
	if err := stage.fixedPhase(ctx, "0006.json", "reloading-manager"); err != nil {
		return err
	}
	if err := systemd.Reload(ctx); err != nil {
		return err
	}
	if err := owned.observeStoppedInstallation(ctx, request); err != nil {
		return err
	}
	if err := owned.observe(ctx, request); err != nil {
		return err
	}
	return stage.fixedPhase(ctx, "0007.json", "installed-stopped")
}
