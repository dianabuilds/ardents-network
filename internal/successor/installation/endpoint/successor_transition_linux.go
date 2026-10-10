package endpoint

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"slices"
	"sort"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// This private preparation retains one native inspection before candidate and
// Release effects. It is not a completed transition or an installed-start proof.
type successorPreparation struct {
	ctx           context.Context
	request       Request
	inspection    *installedRoot
	previous      inspectedGeneration
	terminal      error
	predecessor   *installedPredecessor
	authorization *Authorization
	stage         *installationTransaction
	candidate     *inspectedGeneration
	started       *installedCandidate
	recovery      *successorRecoveryNative
}

func prepareSuccessorNative(ctx context.Context, filename string) (result *successorPreparation, returnedErr error) {
	if ctx == nil {
		return nil, ErrInput
	}
	if err := observeInstallationPlatform(ctx); err != nil {
		return nil, err
	}
	request, err := ReadOwnedRequest(ctx, filename, false)
	if err != nil {
		return nil, err
	}
	reader, err := openInstalledRoot(ctx, request.declared.InstallationRoot)
	if err != nil {
		return nil, err
	}
	owned := &successorPreparation{ctx: ctx, request: request, inspection: reader}
	defer func() {
		if returnedErr != nil {
			owned.terminal = returnedErr
			returnedErr = owned.close()
		}
	}()
	previous, err := reader.inspectSelected(ctx)
	if err != nil {
		return nil, err
	}
	if request.ReleaseHistoryRoot() != previous.binding.ReleaseFloorRoot ||
		request.declared.Headless.NetworkID != previous.request.Headless.NetworkID ||
		!slices.Equal(mutableRoots(request.declared.Headless), mutableRoots(previous.request.Headless)) {
		return nil, ErrBinding
	}
	owned.previous = previous
	if err := owned.observe(); err != nil {
		return nil, err
	}
	return owned, nil
}

func (owned *successorPreparation) observe() error {
	if owned == nil || owned.ctx == nil || owned.inspection == nil {
		return ErrInput
	}
	if err := owned.ctx.Err(); err != nil {
		return err
	}
	if owned.recovery == nil {
		if err := observeRequestCustody(owned.ctx, owned.request); err != nil {
			return err
		}
	} else if owned.recovery.pending != owned || owned.stage == nil || !bytes.Equal(owned.stage.intent.body, owned.recovery.intentBody) {
		return ErrBinding
	}
	if err := owned.inspection.observe(owned.ctx); err != nil {
		return err
	}
	if owned.stage != nil {
		if err := owned.stage.observe(); err != nil {
			return err
		}
	}
	return owned.inspection.observeAccountAndRoots(owned.previous)
}

func (owned *successorPreparation) authenticate(verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs) (Authorization, error) {
	if err := owned.observe(); err != nil {
		return Authorization{}, err
	}
	if owned.authorization != nil || owned.stage != nil {
		return Authorization{}, ErrInput
	}
	if !input.Local.RefTime.Equal(owned.request.ReferenceTime()) {
		return Authorization{}, ErrBinding
	}
	authorized, err := authenticateSuccessor(owned.ctx, verifier, candidate, input, owned.previous.binding)
	if err != nil {
		return Authorization{}, err
	}
	// A late custody change or original cancellation preserves any advanced floors.
	if err := owned.observe(); err != nil {
		return Authorization{}, err
	}
	owned.authorization = &authorized
	return authorized, nil
}

func (owned *successorPreparation) close() error {
	if owned == nil {
		return nil
	}
	if owned.started != nil {
		owned.terminal = errors.Join(owned.terminal, owned.started.close())
		if !owned.started.released {
			// A failed or ambiguous attempt still owns original physical work.
			// Never close the writer lease or stage underneath that lifetime.
			return errors.Join(owned.terminal, ErrBinding)
		}
		// Retain the one-use outcome, including any full ACK, after physical
		// observer close. A copied operation cannot retry or erase admission.
	}
	if owned.stage != nil && owned.stage.intentAttempted && owned.predecessor == nil {
		owned.terminal = errors.Join(owned.terminal, ErrBinding)
		return owned.terminal
	}
	if owned.predecessor != nil {
		if owned.stage != nil && owned.stage.intentAttempted && !owned.predecessor.joined {
			owned.terminal = errors.Join(owned.terminal, owned.predecessor.quiesceAfterFailure())
		} else if (owned.predecessor.stopRequested || owned.predecessor.joinRequired) && !owned.predecessor.joined {
			owned.terminal = errors.Join(owned.terminal, owned.predecessor.joinOriginal())
		}
		if !owned.predecessor.joined && ((owned.stage != nil && owned.stage.intentAttempted) || owned.predecessor.stopRequested || owned.predecessor.joinRequired) {
			// Invalid or incomplete original custody is a retained failure,
			// never permission to free handles underneath unfinished work.
			owned.terminal = errors.Join(owned.terminal, ErrBinding)
			if owned.ctx != nil {
				owned.terminal = errors.Join(owned.terminal, owned.ctx.Err())
			}
			return owned.terminal
		}
		owned.terminal = errors.Join(owned.terminal, owned.predecessor.terminal, owned.predecessor.close())
		owned.predecessor = nil
	}
	if owned.inspection != nil {
		if owned.stage != nil {
			owned.terminal = errors.Join(owned.terminal, owned.stage.close())
			owned.stage = nil
		}
		owned.terminal = errors.Join(owned.terminal, owned.inspection.lease.close())
		owned.inspection = nil
	}
	if owned.ctx != nil {
		owned.terminal = errors.Join(owned.terminal, owned.ctx.Err())
	}
	return owned.terminal
}

func (owned *successorPreparation) stageSuccessor() error {
	if owned == nil || owned.authorization == nil || owned.stage != nil || owned.predecessor == nil {
		return ErrInput
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observe(); err != nil {
		return err
	}
	prepared := preparedInstallation{uid: owned.previous.binding.UID, gid: owned.previous.binding.GID, roots: append([]rootIdentity(nil), owned.previous.binding.MutableRoots...)}
	files, selected, err := assembleGeneration(owned.ctx, owned.request, *owned.authorization, prepared)
	if err != nil {
		return err
	}
	stage, err := stageSuccessorGeneration(owned.ctx, owned.inspection, owned.request, owned.previous.selected, files, selected, prepared.gid)
	// A failed write may already own an intent and physical directory handles.
	// Retain that custody for original-predecessor quiescence; it grants no
	// successful staging or successor admission.
	owned.stage = stage
	if err != nil {
		owned.terminal = errors.Join(owned.terminal, err)
		return err
	}
	if owned.predecessor.joined {
		if err := owned.replaceFixedResources(); err != nil {
			return err
		}
	}
	return owned.observe()
}

func (owned *successorPreparation) publishSuccessorSelection() error {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.joined {
		return ErrBinding
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
		return err
	}
	stage := owned.stage
	if err := stage.verifyTransitionPhase("0004.json", "fixed-resources-replaced"); err != nil {
		return err
	}
	resources, err := successorFixedResources(stage)
	if err != nil {
		return err
	}
	for filename, wanted := range resources {
		if !bytes.Equal(owned.inspection.files[filename].body, wanted) {
			return ErrBinding
		}
	}
	if err := stage.recordSelectionPublication(owned.ctx); err != nil {
		return err
	}
	filename := filepath.Join(stage.lease.path, "selection.json")
	previous := fixedFileObservation{parent: stage.lease.identity, file: owned.inspection.files[filename]}
	body, err := canonicalJSON(stage.selected)
	if err != nil {
		return err
	}
	preimage, err := canonicalJSON(owned.previous.selected)
	if err != nil {
		return err
	}
	if err := stage.recordFixedReplacement(owned.ctx, filename, previous, preimage, body); err != nil {
		return err
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
		return err
	}
	written, err := stage.replaceRecordedFixedFile(owned.ctx, filename, previous, preimage, body)
	if written.identity != nil {
		owned.inspection.files[filename] = written
		stage.fixed[filename] = fixedFileObservation{parent: previous.parent, file: written}
	}
	if err != nil {
		return err
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
		return err
	}
	phase, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: "reloading-manager"})
	if err != nil {
		return err
	}
	if err := stage.journal.Write(owned.ctx, journal.Transitions, "0006.json", phase); err != nil {
		return err
	}
	if err := systemd.Reload(owned.ctx); err != nil {
		return err
	}
	if err := owned.observeReloadedSuccessor(); err != nil {
		return err
	}
	// Reloaded/stopped is a private preparation boundary, never started/READY.
	phase, err = canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: "successor-reloaded-stopped"})
	if err != nil {
		return err
	}
	if err := stage.journal.Write(owned.ctx, journal.Transitions, "0007.json", phase); err != nil {
		return err
	}
	return owned.prepareStartBarrier()
}

func (owned *successorPreparation) observeReloadedSuccessor() error {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.joined || owned.request.declared == nil {
		return ErrBinding
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.stage.verifyTransitionPhase("0006.json", "reloading-manager"); err != nil {
		return err
	}
	version, err := systemd.Version(owned.ctx)
	if err != nil {
		return err
	}
	for range 2 {
		unit, service, err := systemd.Endpoint(owned.ctx)
		if err != nil {
			return err
		}
		if err := verifyQuiescentEndpointProperties(version, unit, service, *owned.request.declared, owned.stage.selected.GenerationDigest); err != nil {
			return err
		}
		if err := observeStoppedActivationSockets(owned.ctx); err != nil {
			return err
		}
		if err := cgroup.ObserveEmpty(owned.ctx); err != nil {
			return err
		}
		if err := observeStoppedManager(owned.ctx); err != nil {
			return err
		}
		if err := owned.observe(); err != nil {
			return err
		}
	}
	// Actual candidate configuration has now been independently observed twice.
	// Keep original process/scope custody and join; update only loaded facts.
	owned.predecessor.loadedRequest = *owned.request.declared
	owned.predecessor.loadedGeneration = owned.stage.selected.GenerationDigest
	return owned.ctx.Err()
}

func (owned *successorPreparation) prepareStartBarrier() error {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil || owned.stage.barrier != nil || owned.predecessor == nil || !owned.predecessor.joined {
		return ErrBinding
	}
	if err := owned.stage.verifyTransitionPhase("0007.json", "successor-reloaded-stopped"); err != nil {
		return err
	}
	if err := owned.observeReloadedSuccessor(); err != nil {
		return err
	}
	if err := owned.inspectStagedCandidate(); err != nil {
		return err
	}
	barrier, err := prepareNativeStartBarrier(owned.ctx, owned.stage)
	owned.stage.barrier = barrier
	if barrier != nil {
		barrier.owner = owned
	}
	if err != nil {
		return err
	}
	if err := owned.observeReloadedSuccessor(); err != nil {
		return err
	}
	return errors.Join(owned.stage.barrier.observe(), owned.ctx.Err())
}

// The still-live fresh-pair owner binds native candidate files before starting
// a process. Stored binding facts remain observations, never Release proofs.
func (owned *successorPreparation) inspectStagedCandidate() error {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.inspection == nil ||
		owned.request.declared == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.joined || owned.candidate != nil {
		return ErrBinding
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.stage.verifyTransitionPhase("0007.json", "successor-reloaded-stopped"); err != nil {
		return err
	}
	binding, files, err := owned.inspection.retainStagedGeneration(owned.ctx, owned.stage)
	if err != nil {
		return err
	}
	selection, err := canonicalJSON(owned.stage.selected)
	if err != nil {
		return err
	}
	checked, err := inspectGeneration(owned.inspection.lease.path, selection, binding, files)
	if err != nil {
		return err
	}
	var intent successorTransitionIntent
	if err := decodeCanonical(owned.stage.intent.body, 128<<10, &intent); err != nil ||
		intent.Schema != "ardents-endpoint-installation-successor-v1" || intent.Candidate != checked.selected {
		return errors.Join(ErrBinding, err)
	}
	wanted, err := canonicalJSON(intent.CandidateBinding)
	if err != nil || !bytes.Equal(binding, wanted) {
		return errors.Join(ErrBinding, err)
	}
	request, err := canonicalJSON(*owned.request.declared)
	if err != nil || !bytes.Equal(files["request.json"], request) {
		return errors.Join(ErrBinding, err)
	}
	if err := owned.inspection.observeAccountAndRoots(checked); err != nil {
		return err
	}
	if err := owned.inspection.inspectFixedResources(owned.ctx, checked); err != nil {
		return err
	}
	if err := owned.observe(); err != nil {
		return err
	}
	owned.candidate = &checked
	return nil
}

// Public facts in the immutable owned intent constrain recovery; they cannot
// replay opaque authorizations. Preserve the accepted successor-v1 identity.
type successorTransitionIntent struct {
	Schema           string              `json:"schema"`
	Previous         generationSelection `json:"previous"`
	Candidate        generationSelection `json:"candidate"`
	CandidateBinding generationBinding   `json:"candidate_binding"`
	Request          installationRequest `json:"request"`
}

func openSuccessor(ctx context.Context, filename string) (*successorPreparation, Request, error) {
	owned, err := prepareSuccessorNative(ctx, filename)
	if err != nil {
		return nil, Request{}, err
	}
	return owned, owned.request, nil
}

func completeSuccessor(owned *successorPreparation, verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs) (result ProvisionResult, returnedErr error) {
	if owned == nil || owned.ctx == nil {
		return ProvisionResult{}, ErrInput
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, owned.ctx.Err())
		if owned.started != nil && owned.started.acknowledged() {
			result = ProvisionResult{Status: "installed-started", GenerationDigest: owned.stage.selected.GenerationDigest}
		}
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			if owned.stage != nil {
				owned.stage.retainFailure(owned.ctx, returnedErr)
			}
		}
	}()
	if _, err := owned.authenticate(verifier, candidate, input); err != nil {
		return ProvisionResult{}, err
	}
	if _, err := owned.retainPredecessor(); err != nil {
		return ProvisionResult{}, err
	}
	if err := owned.stageSuccessor(); err != nil {
		return ProvisionResult{}, err
	}
	// An already-stopped predecessor's staging has completed replacement and
	// barrier preparation. The live path performs those same effects after join.
	if !owned.predecessor.joined {
		if err := owned.predecessor.stopAndJoin(); err != nil {
			return ProvisionResult{}, err
		}
	}
	return ProvisionResult{}, owned.startCandidate()
}

func closeSuccessor(owned *successorPreparation) (error, bool) {
	if owned == nil {
		return nil, true
	}
	first := owned.close()
	if owned.inspection == nil {
		return first, true
	}
	// A command cannot exit and release its separate Release lease while this
	// lifetime still retains unfinished physical work. The first close has
	// already requested any admitted Stop and joined known original scopes.
	// Further observations grant no Stop, ACK or replacement authority.
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	var firstObservation error
	for {
		original := owned.started
		if original != nil && !original.released && !original.acknowledged() {
			joined := original.joined
			if !joined && original.scopes != nil {
				var err error
				joined, err = original.scopes.Joined()
				if err != nil && firstObservation == nil {
					firstObservation = err
				}
				joined = joined && err == nil
			} else if !joined && original.stopRequested {
				// The failed pending-start path may have observed no scope.
				// Only fresh no-Job/stopped/empty facts can complete that attempt.
				joined = true
			}
			if joined {
				err := original.observeQuiescent(context.WithoutCancel(owned.ctx))
				if err != nil && firstObservation == nil {
					firstObservation = err
				}
				if err == nil {
					original.joined = true
					owned.terminal = errors.Join(owned.terminal, firstObservation)
					last := owned.close()
					if owned.inspection == nil {
						return errors.Join(first, last), true
					}
				}
			}
		}
		// Missing original custody remains retained. Cancellation or a waiting
		// timeout cannot manufacture physical completion or release the writer.
		<-ticker.C
	}
}

// Continue an already published selection after fresh proofs and original resync.
// No earlier resource mutation, Stop, startup or ACK is replayed.
func (owned *successorPreparation) resumeReload() error {
	if owned == nil || owned.recovery == nil || owned.authorization == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.alreadyStopped || !owned.predecessor.joined {
		return ErrBinding
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observe(); err != nil {
		return err
	}
	stage := owned.stage
	if err := stage.verifyTransitionPhase("0006.json", "reloading-manager"); err != nil {
		return err
	}
	if err := stage.journal.Resync(owned.ctx, journal.Transitions, "0006.json"); err != nil {
		return err
	}
	if !owned.recovery.inventory["0007.json"] {
		if err := systemd.Reload(owned.ctx); err != nil {
			return err
		}
	}
	if err := owned.observeReloadedSuccessor(); err != nil {
		return err
	}
	if owned.recovery.inventory["0007.json"] {
		if err := stage.verifyTransitionPhase("0007.json", "successor-reloaded-stopped"); err != nil {
			return err
		}
		if err := stage.journal.Resync(owned.ctx, journal.Transitions, "0007.json"); err != nil {
			return err
		}
	} else {
		phase, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: "successor-reloaded-stopped"})
		if err != nil {
			return err
		}
		if err := stage.journal.Write(owned.ctx, journal.Transitions, "0007.json", phase); err != nil {
			return err
		}
	}
	return owned.prepareStartBarrier()
}

// The original fresh-pair owner reaches this only after staging and real
// quiescence. Keep every preimage unchanged while establishing repair provenance.
func (owned *successorPreparation) recordFixedReplacements() (returnedErr error) {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.joined {
		return ErrBinding
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			owned.stage.retainFailure(owned.ctx, returnedErr)
		}
	}()
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
		return err
	}
	stage := owned.stage
	if len(stage.journal.Bytes(journal.Transitions, "0003.json")) == 0 {
		if len(stage.journal.Bytes(journal.Transitions, "0002.json")) == 0 {
			return ErrBinding
		}
		body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: stage.selected.GenerationDigest, BindingDigest: stage.selected.BindingDigest, Phase: "replacing-fixed-resources"})
		if err != nil {
			return err
		}
		if err := stage.journal.Write(owned.ctx, journal.Transitions, "0003.json", body); err != nil {
			return err
		}
	}
	resources, err := successorFixedResources(stage)
	if err != nil {
		return err
	}
	preimages, err := recoveryFixedBytes(owned.previous)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(resources))
	for filename := range resources {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	for _, filename := range paths {
		if err := owned.observe(); err != nil {
			return err
		}
		if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
			return err
		}
		previous := fixedFileObservation{parent: owned.inspection.directories[filepath.Dir(filename)], file: owned.inspection.files[filename]}
		if err := stage.recordFixedReplacement(owned.ctx, filename, previous, preimages[filename], resources[filename]); err != nil {
			return err
		}
	}
	return errors.Join(owned.observe(), owned.predecessor.observeQuiescent(owned.ctx))
}

// This operation retains the same fresh-pair owner, original lease and joined
// predecessor. Filesystem provenance never substitutes for those admissions.
func (owned *successorPreparation) replaceFixedResources() (returnedErr error) {
	if owned == nil || owned.ctx == nil || owned.authorization == nil || owned.stage == nil || owned.predecessor == nil || !owned.predecessor.joined {
		return ErrBinding
	}
	defer func() {
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			if owned.stage.terminal == nil {
				owned.stage.retainFailure(owned.ctx, returnedErr)
			}
		}
	}()
	if err := owned.recordFixedReplacements(); err != nil {
		return err
	}
	resources, err := successorFixedResources(owned.stage)
	if err != nil {
		return err
	}
	preimages, err := recoveryFixedBytes(owned.previous)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(resources))
	for filename := range resources {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	for _, filename := range paths {
		if err := owned.observe(); err != nil {
			return err
		}
		if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
			return err
		}
		previous := fixedFileObservation{parent: owned.inspection.directories[filepath.Dir(filename)], file: owned.inspection.files[filename]}
		written, err := owned.stage.replaceRecordedFixedFile(owned.ctx, filename, previous, preimages[filename], resources[filename])
		if written.identity != nil {
			owned.inspection.files[filename] = written
			owned.stage.fixed[filename] = fixedFileObservation{parent: previous.parent, file: written}
		}
		if err != nil {
			return err
		}
	}
	if err := owned.observe(); err != nil {
		return err
	}
	if err := owned.predecessor.observeQuiescent(owned.ctx); err != nil {
		return err
	}
	if err := owned.stage.recordFixedReplacementCompletion(owned.ctx); err != nil {
		return err
	}
	return owned.publishSuccessorSelection()
}
