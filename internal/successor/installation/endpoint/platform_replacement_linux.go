package endpoint

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	endpointunit "github.com/dianabuilds/ardents-network/internal/successor/installation/unit"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// StagePlatformSuccessor seals a different generation in a new immutable
// root. The platform must retain the original unit across its Stop so its
// successful exit is still observable. No manager job or resource replacement
// is performed here. The caller retains the existing Release verifier lease
// throughout, excluding another platform pre-start authorization.
func StagePlatformSuccessor(ctx context.Context, request Request, candidate enrollment.Candidate, input release.Inputs, verifier *release.Verifier,
	previousRoot, checkProgram, manifest, pin string) (returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || request.declared == nil || verifier == nil ||
		!canonicalPath(previousRoot) || previousRoot == "/" ||
		!input.Local.RefTime.Equal(request.ReferenceTime()) {
		return ErrInput
	}
	directory := request.declared.InstallationRoot
	if directory == previousRoot || strings.HasPrefix(directory, previousRoot+"/") || strings.HasPrefix(previousRoot, directory+"/") {
		return ErrBinding
	}
	if _, err := os.Lstat(directory); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(ErrBinding, err)
	}
	if _, err := rootDirectoryAncestors(filepath.Dir(directory)); err != nil {
		return err
	}
	reference, err := systemd.RetainEndpoint(ctx)
	defer func() { returnedErr = errors.Join(returnedErr, reference.Close()) }()
	if err != nil {
		return err
	}
	ancestors, err := rootDirectoryAncestors(filepath.Dir(previousRoot))
	if err != nil {
		return err
	}
	uid, gid, err := observeEndpointAccount()
	if err != nil || uid != gid || uid < 100 || uid > 999 {
		return errors.Join(ErrNativeUnavailable, err)
	}
	reader := &installedFiles{directory: previousRoot, gid: gid, files: make(map[string]fileObservation),
		directories: ancestors, mutableDirectories: make(map[string]os.FileInfo)}
	if err := reader.pinGenerationDirectory(previousRoot); err != nil {
		return err
	}
	previous, err := inspectPlatformPredecessor(ctx, reader)
	if err != nil {
		return err
	}
	roots := mutableRoots(previous.request.Headless)
	if previous.binding.UID != uid || previous.binding.GID != gid ||
		request.ReleaseHistoryRoot() != previous.binding.ReleaseFloorRoot ||
		request.declared.Headless.NetworkID != previous.request.Headless.NetworkID ||
		!slices.Equal(mutableRoots(request.declared.Headless), roots) {
		return ErrBinding
	}
	facts, valid := candidate.Facts()
	program, present := candidate.File(facts.Artifact)
	if !valid || !present || previous.binding.Program.Digest == digestHex(program) {
		return ErrAuthorization
	}
	if _, err := expectedUnit(*request.declared); err != nil {
		return err
	}
	expected, err := expectedUnit(previous.request)
	if err != nil {
		return err
	}
	version, err := systemd.Version(ctx)
	if err != nil {
		return err
	}
	observe := func() error {
		if err := errors.Join(observeRequestCustody(ctx, request), refusePlatformTransitionRecords(previousRoot),
			reader.observe(ctx), reader.observeAccountAndRoots(previous), reference.Observe(ctx)); err != nil {
			return err
		}
		unit, service, err := systemd.Endpoint(ctx)
		if err != nil {
			return err
		}
		if err := endpointunit.VerifyPlatformCompleted(version, unit, service, expected,
			previous.binding.GenerationDigest, checkProgram, manifest, pin); err != nil {
			return err
		}
		if err := observeStoppedActivationSockets(ctx); err != nil {
			return err
		}
		for _, role := range []string{"reader", "publisher"} {
			unit, _, err := systemd.Activation(ctx, role)
			if err != nil {
				return err
			}
			if err := endpointunit.VerifyNoJob(unit); err != nil {
				return err
			}
		}
		return errors.Join(cgroup.ObserveEmpty(ctx), reference.Observe(ctx))
	}
	if err := observe(); err != nil {
		return err
	}
	authorization, err := authenticateSuccessor(ctx, verifier, candidate, input, previous.binding)
	if err != nil {
		return err
	}
	prepared := preparedInstallation{uid: uid, gid: gid, roots: previous.binding.MutableRoots}
	return sealPlatformGeneration(ctx, request, authorization, verifier, prepared, observe)
}

func inspectPlatformPredecessor(ctx context.Context, reader *installedFiles) (inspectedGeneration, error) {
	if err := refusePlatformTransitionRecords(reader.directory); err != nil {
		return inspectedGeneration{}, err
	}
	selectedRaw, err := reader.read(ctx, filepath.Join(reader.directory, "selection.json"), 4<<10, 0640, reader.gid)
	if err != nil {
		return inspectedGeneration{}, err
	}
	var selected generationSelection
	if decodeCanonical(selectedRaw, 4<<10, &selected) != nil || !canonicalDigest(selected.GenerationDigest) {
		return inspectedGeneration{}, ErrBinding
	}
	parent := filepath.Join(reader.directory, "generations")
	for _, name := range []string{parent, filepath.Join(parent, selected.GenerationDigest)} {
		if err := reader.pinGenerationDirectory(name); err != nil {
			return inspectedGeneration{}, err
		}
	}
	binding, files, err := reader.readSealedGeneration(ctx, selected.GenerationDigest)
	if err != nil {
		return inspectedGeneration{}, err
	}
	return inspectGeneration(reader.directory, selectedRaw, binding, files)
}

func refusePlatformTransitionRecords(directory string) error {
	for _, name := range []string{"writer.lock", "transition.json", "start-guard.json", "start-socket.json", "start-completion.socket", "journals", "preparation"} {
		if _, err := os.Lstat(filepath.Join(directory, name)); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	return nil
}
