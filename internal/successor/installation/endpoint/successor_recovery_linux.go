package endpoint

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"syscall"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	generationauthorization "github.com/dianabuilds/ardents-network/internal/successor/installation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/fixedfile"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/generation"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
)

// This recovery owns fresh independent read/process/scope observations. It does
// not adopt a creation Owner, stage, listener or a former caller's ACK latch.
type successorRecoveryNative struct {
	reader             *installedRoot
	intent             successorTransitionIntent
	intentBody         []byte
	checked            inspectedGeneration
	previous           inspectedGeneration
	journal            string
	inventory          map[string]bool
	removals           [3]completionRemoval
	process            *installedProcessPin
	scopes             *cgroup.Lifetime
	pending            *successorPreparation
	stagedReplacements *journal.RetainedReplacements
	fixedMutation      bool
	writing            *generationWriteRecovery
}

func openSuccessorRecovery(ctx context.Context, root string, reference time.Time) (result *successorRecoveryNative, request installationRequest, returnedErr error) {
	if err := observeInstallationPlatform(ctx); err != nil {
		return nil, request, err
	}
	reader, err := openInstalledRoot(ctx, root)
	if err != nil {
		return nil, request, errors.Join(ErrRepairRequired, err)
	}
	r := &successorRecoveryNative{reader: reader}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(ErrRepairRequired, returnedErr, closeSuccessorRecovery(r))
		}
	}()
	for _, name := range []string{"preparation/failure.json"} {
		if _, err := reader.lease.root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			return nil, request, errors.Join(ErrBinding, err)
		}
	}
	_, pendingErr := reader.lease.root.Lstat("transition.json")
	if pendingErr != nil && !errors.Is(pendingErr, os.ErrNotExist) {
		return nil, request, pendingErr
	}
	pending := pendingErr == nil
	selectedBody, err := reader.readObserved(ctx, filepath.Join(root, "selection.json"), 4<<10, 0640, reader.gid, pending)
	var selected generationSelection
	if err != nil || !pending && (decodeCanonical(selectedBody, 4<<10, &selected) != nil || selected.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(selected.GenerationDigest) || !canonicalDigest(selected.BindingDigest)) {
		return nil, request, errors.Join(ErrBinding, err)
	}
	if pending {
		r.intentBody, err = reader.read(ctx, filepath.Join(root, "transition.json"), 128<<10, 0600, 0)
		if err != nil {
			return nil, request, err
		}
	} else {
		r.intentBody, err = reader.read(ctx, filepath.Join(root, "journals", selected.GenerationDigest, "completed-intent.json"), 128<<10, 0600, 0)
		if err != nil {
			return nil, request, err
		}
	}
	if decodeCanonical(r.intentBody, 128<<10, &r.intent) != nil || r.intent.Schema != "ardents-endpoint-installation-successor-v1" || r.intent.Candidate.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(r.intent.Candidate.GenerationDigest) || !canonicalDigest(r.intent.Candidate.BindingDigest) || r.intent.Previous.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(r.intent.Previous.GenerationDigest) || !canonicalDigest(r.intent.Previous.BindingDigest) || r.intent.Previous.GenerationDigest == r.intent.Candidate.GenerationDigest || !pending && r.intent.Candidate != selected {
		return nil, request, errors.Join(ErrBinding, err)
	}
	r.journal = filepath.Join(root, "journals", r.intent.Candidate.GenerationDigest)
	request = r.intent.Request
	requestBody, err := canonicalJSON(request)
	if err != nil {
		return nil, request, err
	}
	if _, err := DecodeRequest(ctx, requestBody, false); err != nil {
		return nil, request, err
	}
	original, err := time.Parse(time.RFC3339Nano, request.ReferenceTime)
	if err != nil || reference.Before(original) || request.InstallationRoot != root {
		return nil, request, ErrBinding
	}
	if pending {
		_, phaseErr := os.Lstat(filepath.Join(r.journal, "0002.json"))
		failedPhase := false
		if phaseErr == nil {
			body, err := reader.read(ctx, filepath.Join(r.journal, "0002.json"), 64<<10, 0600, 0)
			var phase generationTransition
			if err != nil || decodeCanonical(body, 64<<10, &phase) != nil {
				return nil, request, errors.Join(ErrBinding, err)
			}
			failedPhase = r.matchesGenerationFailure(phase)
		}
		if errors.Is(phaseErr, os.ErrNotExist) || failedPhase {
			r.writing = &generationWriteRecovery{births: make(map[string]generationFileBirth)}
			if err := r.openWritingTransition(ctx); err != nil {
				return nil, request, err
			}
			return r, request, nil
		}
		if phaseErr != nil {
			return nil, request, phaseErr
		}
	}
	for index, selection := range []generationSelection{r.intent.Candidate, r.intent.Previous} {
		parent, directory := filepath.Join(root, "generations"), filepath.Join(root, "generations", selection.GenerationDigest)
		for _, path := range []string{parent, directory} {
			if err := reader.pinGenerationDirectory(path); err != nil {
				return nil, request, err
			}
		}
		binding, files, err := reader.readSealedGeneration(ctx, selection.GenerationDigest)
		selectionBody, encodeErr := canonicalJSON(selection)
		checked, inspectErr := inspectGeneration(root, selectionBody, binding, files)
		if err := errors.Join(err, encodeErr, inspectErr); err != nil {
			return nil, request, err
		}
		if index == 0 {
			r.checked = checked
		} else {
			r.previous = checked
		}
	}
	binding, err := canonicalJSON(r.intent.CandidateBinding)
	checkedBinding, checkedErr := canonicalJSON(r.checked.binding)
	checkedRequest, requestErr := canonicalJSON(r.checked.request)
	if err != nil || checkedErr != nil || requestErr != nil || !bytes.Equal(binding, checkedBinding) || !bytes.Equal(requestBody, checkedRequest) || request.ReleaseFloorRoot != r.checked.binding.ReleaseFloorRoot || request.ReleaseFloorRoot != r.previous.binding.ReleaseFloorRoot ||
		r.checked.binding.UID != r.previous.binding.UID || r.checked.binding.GID != reader.gid || r.previous.binding.GID != reader.gid || !slices.Equal(r.checked.binding.MutableRoots, r.previous.binding.MutableRoots) || r.checked.request.Headless.NetworkID != r.previous.request.Headless.NetworkID {
		return nil, request, errors.Join(ErrBinding, err, checkedErr, requestErr)
	}
	if err := reader.observeAccountAndRoots(r.checked); err != nil {
		return nil, request, err
	}
	before, after := r.previous.binding.Generation, r.checked.binding.Generation
	if after.ReleaseVersion <= before.ReleaseVersion || after.Platform != before.Platform || after.Architecture != before.Architecture || after.Environment != before.Environment || after.Network != before.Network {
		return nil, request, ErrBinding
	}
	if pending {
		if err := r.readStagedFixedResources(ctx); err != nil {
			return nil, request, err
		}
	} else if err := reader.inspectFixedResources(ctx, r.checked); err != nil {
		return nil, request, err
	}
	if err := r.readPreparation(ctx); err != nil {
		return nil, request, err
	}
	if pending {
		if err := r.retainStagedTransition(ctx); err != nil {
			return nil, request, err
		}
		return r, request, nil
	}
	if err := r.readJournal(ctx); err != nil {
		return nil, request, err
	}
	if err := r.readReplacements(ctx); err != nil {
		return nil, request, err
	}
	if err := r.readRemovals(ctx); err != nil {
		return nil, request, err
	}
	if err := r.retainInvocation(ctx); err != nil {
		return nil, request, err
	}
	if err := r.observe(ctx); err != nil {
		return nil, request, err
	}
	return r, request, nil
}

func (r *successorRecoveryNative) readPreparation(ctx context.Context) error {
	directory := filepath.Join(r.reader.lease.path, "preparation")
	if err := recoveryInventory(directory, map[string]bool{"0001.json": true, "0002.json": true, "0003.json": true}); err != nil {
		return err
	}
	binding := r.checked.binding
	if r.writing != nil {
		binding = r.intent.CandidateBinding
	}
	var previous journal.Record
	for _, name := range []string{"0001.json", "0002.json", "0003.json"} {
		body, err := r.reader.read(ctx, filepath.Join(directory, name), 64<<10, 0600, 0)
		var current journal.Record
		if err != nil || decodeCanonical(body, 64<<10, &current) != nil {
			return errors.Join(ErrBinding, err)
		}
		next, err := journal.Next(previous, current)
		if err != nil || next != name || name != "0001.json" && (current.UID != binding.UID || current.GID != binding.GID) {
			return errors.Join(ErrBinding, err)
		}
		previous = current
	}
	return nil
}

func (r *successorRecoveryNative) readJournal(ctx context.Context) error {
	phases := []string{"writing-generation", "generation-staged", "replacing-fixed-resources", "fixed-resources-replaced", "publishing-selection", "reloading-manager", "successor-reloaded-stopped"}
	r.inventory = map[string]bool{"generation-directory.json": true, "replacements": true, "completed-intent.json": true, "started-invocation.json": true,
		"completion-socket-removal.json": true, "completion-record-removal.json": true, "completion-guard-removal.json": true,
		"original-transition-failure.json": false, "original-transition-failure-archived.json": false, "start-attempt.json": false}
	for index, phase := range phases {
		name := "000" + string(rune('1'+index)) + ".json"
		r.inventory[name] = true
		body, err := r.reader.read(ctx, filepath.Join(r.journal, name), 64<<10, 0600, 0)
		var record generationTransition
		if err != nil || decodeCanonical(body, 64<<10, &record) != nil || record.Schema != "ardents-endpoint-installation-transition-v1" || record.GenerationDigest != r.intent.Candidate.GenerationDigest || record.BindingDigest != r.intent.Candidate.BindingDigest || record.Phase != phase || record.OriginalError != "" {
			return errors.Join(ErrBinding, err)
		}
	}
	if _, err := readGenerationFileBirths(ctx, r.reader, r.journal, r.intent.Candidate, r.checked.binding, r.checked.files, r.inventory); err != nil {
		return err
	}
	if err := recoveryInventory(r.journal, r.inventory); err != nil {
		return err
	}
	for _, name := range []string{"original-transition-failure.json", "original-transition-failure-archived.json"} {
		body, err := r.reader.read(ctx, filepath.Join(r.journal, name), 64<<10, 0600, 0)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		var record generationTransition
		if err != nil || decodeCanonical(body, 64<<10, &record) != nil || record.Schema != "ardents-endpoint-installation-transition-v1" || record.GenerationDigest != r.intent.Candidate.GenerationDigest || record.BindingDigest != r.intent.Candidate.BindingDigest || (record.Phase != "successor-transition-failed" && record.Phase != "generation-write-failed") || record.OriginalError == "" {
			return errors.Join(ErrBinding, err)
		}
	}
	live := r.reader.files[filepath.Join(r.journal, "original-transition-failure.json")]
	archived := r.reader.files[filepath.Join(r.journal, "original-transition-failure-archived.json")]
	if live.identity != nil && archived.identity != nil && !bytes.Equal(live.body, archived.body) {
		return ErrBinding
	}
	body, err := r.reader.read(ctx, filepath.Join(r.journal, "generation-directory.json"), 4<<10, 0600, 0)
	var birth generationBirth
	if err != nil || decodeCanonical(body, 4<<10, &birth) != nil || birth.Schema != "ardents-endpoint-generation-directory-v1" || birth.GenerationDigest != r.intent.Candidate.GenerationDigest {
		return errors.Join(ErrBinding, err)
	}
	info := r.reader.directories[filepath.Join(r.reader.lease.path, "generations", birth.GenerationDigest)].Sys().(*syscall.Stat_t)
	if birth.Device != uint64(info.Dev) || birth.Inode != info.Ino {
		return ErrBinding
	}
	return nil
}

func recoveryFixedBytes(checked inspectedGeneration) (map[string][]byte, error) {
	resources, err := fixedResourceImages(checked.files)
	if err != nil {
		return nil, err
	}
	resources[filepath.Join(checked.binding.InstallationRoot, "selection.json")], err = canonicalJSON(checked.selected)
	return resources, err
}

func (r *successorRecoveryNative) readReplacements(ctx context.Context) error {
	before, err := recoveryFixedBytes(r.previous)
	if err != nil {
		return err
	}
	after, err := recoveryFixedBytes(r.checked)
	if err != nil {
		return err
	}
	directory := filepath.Join(r.journal, "replacements")
	allowed := make(map[string]bool, len(after))
	for filename := range after {
		allowed[digestHex([]byte(filename))+".json"] = true
	}
	if err := recoveryInventory(directory, allowed); err != nil {
		return err
	}
	for filename, body := range after {
		name := digestHex([]byte(filename)) + ".json"
		recorded, err := r.reader.read(ctx, filepath.Join(directory, name), 4<<10, 0600, 0)
		var record fixedReplacementRecord
		observed := r.reader.files[filename]
		if err != nil || decodeCanonical(recorded, 4<<10, &record) != nil || observed.identity == nil || record.Schema != "ardents-endpoint-installation-replacement-v1" || record.GenerationDigest != r.intent.Candidate.GenerationDigest || record.Path != filename || record.PreviousDigest != digestHex(before[filename]) || record.CandidateDigest != digestHex(body) || record.Mode != uint32(observed.mode) || record.GID != observed.gid || !bytes.Equal(observed.body, body) {
			return errors.Join(ErrBinding, err)
		}
		native := observed.identity.Sys().(*syscall.Stat_t)
		if record.Device == 0 || record.Device != uint64(native.Dev) || record.Inode == 0 || record.Inode != native.Ino {
			return ErrBinding
		}
	}
	return nil
}

func (r *successorRecoveryNative) retainInvocation(ctx context.Context) error {
	body, err := r.reader.read(ctx, filepath.Join(r.journal, "started-invocation.json"), 4<<10, 0600, 0)
	var record startedInvocationObservation
	if err != nil || decodeCanonical(body, 4<<10, &record) != nil || record.Schema != "ardents-endpoint-installation-start-observation-v1" || record.GenerationDigest != r.intent.Candidate.GenerationDigest || record.BindingDigest != r.intent.Candidate.BindingDigest || record.PID == 0 {
		return errors.Join(ErrBinding, err)
	}
	encoded, err := hex.DecodeString(record.Invocation)
	if err != nil || len(encoded) != 16 || hex.EncodeToString(encoded) != record.Invocation {
		return ErrBinding
	}
	var invocation [16]byte
	copy(invocation[:], encoded)
	if invocation == [16]byte{} {
		return ErrBinding
	}
	r.process, err = pinInstalledProcess(ctx, r.reader, r.checked, record.PID, invocation)
	if err != nil {
		return err
	}
	r.scopes, err = cgroup.Retain(ctx, record.PID, r.checked.binding.UID)
	return err
}

func (r *successorRecoveryNative) observe(ctx context.Context) error {
	if r == nil || r.reader == nil || r.process == nil || r.scopes == nil {
		return ErrBinding
	}
	if err := recoveryInventory(r.journal, r.inventory); err != nil {
		return err
	}
	if err := r.reader.observe(ctx); err != nil {
		return err
	}
	if err := r.reader.observeAccountAndRoots(r.checked); err != nil {
		return err
	}
	if err := r.observeRemovals(ctx); err != nil {
		return err
	}
	for range 2 {
		version, err := systemd.Version(ctx)
		if err != nil {
			return err
		}
		unit, service, err := systemd.Endpoint(ctx)
		if err != nil {
			return err
		}
		if err := verifyRunningEndpointProperties(version, unit, service, r.checked.request, r.checked.selected.GenerationDigest, r.process.pid, r.process.invocation); err != nil {
			return err
		}
		if err := observeBoundActivationSockets(ctx); err != nil {
			return err
		}
		if err := r.process.observeOriginalProcess(ctx); err != nil {
			return err
		}
		if err := r.scopes.Observe(); err != nil {
			return err
		}
	}
	return r.reader.observe(ctx)
}

func completeSuccessorRecovery(ctx context.Context, r *successorRecoveryNative, reference time.Time, verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs) (ProvisionResult, error) {
	if verifier == nil {
		return ProvisionResult{}, ErrInput
	}
	if !input.Local.RefTime.Equal(reference) {
		return ProvisionResult{}, ErrBinding
	}
	if r.writing != nil {
		return r.completeWritingTransition(ctx, reference, verifier, candidate, input)
	}
	if r.pending != nil {
		return r.completeStagedTransition(ctx, reference, verifier, candidate, input)
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := successorContinuity(r.checked.binding, floors, input.Local); err != nil {
		return ProvisionResult{}, err
	}
	authorization, err := generationauthorization.AuthenticateCandidate(ctx, verifier, candidate, input)
	if err != nil {
		return ProvisionResult{}, err
	}
	checked, err := recoverBoundGeneration(ctx, r.intent.Candidate, r.intent.CandidateBinding, r.intent.Request, reference, authorization)
	if err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	for name, body := range checked.files {
		if !bytes.Equal(body, r.checked.files[name]) {
			return ProvisionResult{}, errors.Join(ErrRepairRequired, ErrBinding)
		}
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	// Resynchronize every original private provenance record before cleanup.
	metadata := make([]string, 0, len(r.reader.files))
	for filename, observed := range r.reader.files {
		if observed.mode == 0600 && observed.gid == 0 {
			metadata = append(metadata, filename)
		}
	}
	sort.Strings(metadata)
	for _, filename := range metadata {
		if err := r.reader.syncObserved(ctx, filename); err != nil {
			return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
		}
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	if err := r.finishRemoval(ctx); err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	// The exact first failure is copied durably before its original is removed.
	source := filepath.Join(r.journal, "original-transition-failure.json")
	if original, exists := r.reader.files[source]; exists {
		if err := r.reader.writePrivate(ctx, filepath.Join(r.journal, "original-transition-failure-archived.json"), original.body); err != nil {
			return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
		}
		if err := r.observe(ctx); err != nil {
			return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
		}
		if err := r.reader.removeObserved(ctx, source); err != nil {
			return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
		}
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	return ProvisionResult{Status: "installed-recovered-started", GenerationDigest: checked.selected.GenerationDigest}, ctx.Err()
}

func closeSuccessorRecovery(r *successorRecoveryNative) error {
	if r == nil {
		return nil
	}
	var err error
	if r.pending != nil {
		err, _ = closeSuccessor(r.pending)
		return err
	}
	if r.scopes != nil {
		err = errors.Join(err, r.scopes.Close())
	}
	if r.process != nil {
		err = errors.Join(err, r.process.close())
	}
	if r.reader != nil {
		err = errors.Join(err, r.reader.lease.close())
	}
	return err
}

// Public expected facts and actual incomplete images remain separate from an
// inspected candidate and fresh private proofs throughout phase1 admission.
type generationWriteRecovery struct {
	births      map[string]generationFileBirth
	failedPhase bool
}

func (r *successorRecoveryNative) matchesGenerationFailure(record generationTransition) bool {
	return record.Schema == "ardents-endpoint-installation-transition-v1" &&
		record.GenerationDigest == r.intent.Candidate.GenerationDigest &&
		record.BindingDigest == r.intent.Candidate.BindingDigest &&
		record.Phase == "generation-write-failed" && record.OriginalError != ""
}

// Only the original generation-writing failure may accompany a pending prefix.
// Start failures and detached error text cannot grant another attempted start.
func (r *successorRecoveryNative) readGenerationFailure(ctx context.Context, records map[string]journal.RetainedRecord) error {
	name := "original-transition-failure.json"
	filename := filepath.Join(r.journal, name)
	body, err := r.reader.read(ctx, filename, 64<<10, 0600, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	var record generationTransition
	if err != nil || decodeCanonical(body, 64<<10, &record) != nil || !r.matchesGenerationFailure(record) {
		return errors.Join(ErrBinding, err)
	}
	r.inventory[name] = true
	records[name] = journal.RetainedRecord{Identity: r.reader.files[filename].identity, Bytes: body}
	return nil
}

func (r *successorRecoveryNative) openWritingTransition(ctx context.Context) error {
	parent := filepath.Join(r.reader.lease.path, "generations")
	previousDirectory := filepath.Join(parent, r.intent.Previous.GenerationDigest)
	for _, directory := range []string{parent, previousDirectory} {
		if err := r.reader.pinGenerationDirectory(directory); err != nil {
			return err
		}
	}
	previousBinding, previousFiles, err := r.reader.readSealedGeneration(ctx, r.intent.Previous.GenerationDigest)
	previousSelection, encodeErr := canonicalJSON(r.intent.Previous)
	previous, inspectErr := inspectGeneration(r.reader.lease.path, previousSelection, previousBinding, previousFiles)
	if err := errors.Join(err, encodeErr, inspectErr); err != nil {
		return err
	}
	r.previous = previous
	if err := r.validateWritingBinding(); err != nil {
		return err
	}
	if err := r.reader.observeAccountAndRoots(previous); err != nil {
		return err
	}
	if err := r.reader.inspectFixedResources(ctx, previous); err != nil {
		return err
	}
	if err := r.readPreparation(ctx); err != nil {
		return err
	}
	for _, name := range []string{"start-guard.json", "start-completion.socket", "start-socket.json"} {
		if _, err := r.reader.lease.root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	expectedSelection, err := canonicalJSON(r.intent.Previous)
	if err != nil || !bytes.Equal(r.reader.files[filepath.Join(r.reader.lease.path, "selection.json")].body, expectedSelection) {
		return errors.Join(ErrBinding, err)
	}
	records, images, err := r.readWritingJournal(ctx)
	if err != nil {
		return err
	}
	requestBody, err := canonicalJSON(r.intent.Request)
	if err != nil {
		return err
	}
	request, err := DecodeRequest(ctx, requestBody, false)
	if err != nil {
		return err
	}
	directory := filepath.Join(parent, r.intent.Candidate.GenerationDigest)
	stage := &installationTransaction{lease: r.reader.lease, selected: r.intent.Candidate, intent: r.reader.files[filepath.Join(r.reader.lease.path, "transition.json")], fixed: make(map[string]fixedFileObservation)}
	owned := &successorPreparation{ctx: ctx, request: request, inspection: r.reader, previous: previous, stage: stage, recovery: r}
	r.pending = owned // Register partial custody before reopening any descriptor.
	stage.prefix, err = generation.OpenPrefix(ctx, parent, r.reader.directories[parent], r.reader.directories[directory], r.intent.Candidate.GenerationDigest, r.reader.gid, images)
	if err != nil {
		return err
	}
	stage.journal, err = journal.OpenTransition(ctx, filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records)
	if err != nil {
		return err
	}
	selection := filepath.Join(r.reader.lease.path, "selection.json")
	stage.fixed[selection] = fixedFileObservation{parent: r.reader.lease.identity, file: r.reader.files[selection]}
	return owned.observe()
}

// Local intent facts constrain admission but never stand in for inspected bytes.
func (r *successorRecoveryNative) validateWritingBinding() error {
	b, old, request := r.intent.CandidateBinding, r.previous.binding, r.intent.Request
	encoded, err := canonicalJSON(b)
	if err != nil || len(encoded) > 32<<10 || digestHex(encoded) != r.intent.Candidate.BindingDigest || b.Schema != "ardents-endpoint-installation-binding-v1" || b.InstallationRoot != r.reader.lease.path || b.GenerationDigest != r.intent.Candidate.GenerationDigest || b.Account != "ardents-endpoint" || b.Unit != "ardents-endpoint.service" || b.UID == 0 || b.UID == ^uint32(0) || b.UID != old.UID || b.GID != r.reader.gid || b.GID != old.GID || b.ReleaseFloorRoot != request.ReleaseFloorRoot || b.ReleaseFloorRoot != old.ReleaseFloorRoot || !slices.Equal(b.MutableRoots, old.MutableRoots) || !slices.Equal(mutableRoots(request.Headless), mutableRoots(r.previous.request.Headless)) || request.Headless.NetworkID != r.previous.request.Headless.NetworkID || len(b.Files) != 14 {
		return errors.Join(ErrBinding, err)
	}
	for _, name := range generation.Names() {
		if name != "binding.json" && !canonicalDigest(b.Files[name]) {
			return ErrBinding
		}
	}
	program, err := b.Program.observation()
	if err != nil {
		return err
	}
	target, err := b.Generation.observation()
	if err != nil || !generationauthorization.CoherentTargets(program, target, generationDeclaration{Platform: b.Generation.Platform, ReleaseIdentity: b.Generation.ReleaseIdentity, ReleaseVersion: b.Generation.ReleaseVersion}) || program.Platform != "linux-amd64" || program.Architecture != "amd64" || b.Program.ReferenceTime != request.ReferenceTime || b.Generation.ReferenceTime != request.ReferenceTime || b.Generation.Digest != r.intent.Candidate.GenerationDigest || b.Files["protected-endpoint.json"] != b.Generation.Digest || b.Files["ardents-linux-amd64"] != b.Program.Digest || b.Program.Length > 64<<20 || b.Generation.Length > 64<<20 {
		return errors.Join(ErrBinding, err)
	}
	before, after := old.Generation, b.Generation
	if after.ReleaseVersion <= before.ReleaseVersion || after.Platform != before.Platform || after.Architecture != before.Architecture || after.Environment != before.Environment || after.Network != before.Network {
		return ErrBinding
	}
	return nil
}

// The root owns closed phase/order/provenance admission. All preceding recorded
// leaves must be complete; only the last birth may have a private/torn image.
func (r *successorRecoveryNative) readWritingJournal(ctx context.Context) (map[string]journal.RetainedRecord, map[string]generation.PrefixFile, error) {
	if r.writing == nil {
		return nil, nil, ErrBinding
	}
	records := make(map[string]journal.RetainedRecord)
	r.inventory = map[string]bool{"0001.json": true, "generation-directory.json": true, "generation-files.json": true}
	if err := r.readGenerationFailure(ctx, records); err != nil {
		return nil, nil, err
	}
	failedPath := filepath.Join(r.journal, "0002.json")
	if body, err := r.reader.read(ctx, failedPath, 64<<10, 0600, 0); !errors.Is(err, os.ErrNotExist) {
		var failure generationTransition
		if err != nil || decodeCanonical(body, 64<<10, &failure) != nil || !r.matchesGenerationFailure(failure) {
			return nil, nil, errors.Join(ErrBinding, err)
		}
		if first, present := records["original-transition-failure.json"]; present && !bytes.Equal(first.Bytes, body) {
			return nil, nil, ErrBinding
		}
		r.writing.failedPhase = true
		r.inventory["0002.json"] = true
		records["0002.json"] = journal.RetainedRecord{Identity: r.reader.files[failedPath].identity, Bytes: body}
	}
	phasePath := filepath.Join(r.journal, "0001.json")
	phase, err := r.reader.read(ctx, phasePath, 64<<10, 0600, 0)
	var observed generationTransition
	if err != nil || decodeCanonical(phase, 64<<10, &observed) != nil || observed != (generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, BindingDigest: r.intent.Candidate.BindingDigest, Phase: "writing-generation"}) {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	records["0001.json"] = journal.RetainedRecord{Identity: r.reader.files[phasePath].identity, Bytes: phase}
	markerPath := filepath.Join(r.journal, "generation-files.json")
	marker, err := r.reader.read(ctx, markerPath, 4<<10, 0600, 0)
	wanted, encodeErr := canonicalJSON(generationFileInventory{Schema: "ardents-endpoint-generation-files-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, BindingDigest: r.intent.Candidate.BindingDigest, Files: generation.Names()})
	if err != nil || encodeErr != nil || !bytes.Equal(marker, wanted) {
		return nil, nil, errors.Join(ErrBinding, err, encodeErr)
	}
	records["generation-files.json"] = journal.RetainedRecord{Identity: r.reader.files[markerPath].identity, Bytes: marker}
	birthPath := filepath.Join(r.journal, "generation-directory.json")
	body, err := r.reader.read(ctx, birthPath, 4<<10, 0600, 0)
	var directoryBirth generationBirth
	if err != nil || decodeCanonical(body, 4<<10, &directoryBirth) != nil || directoryBirth.Schema != "ardents-endpoint-generation-directory-v1" || directoryBirth.GenerationDigest != r.intent.Candidate.GenerationDigest || directoryBirth.Device == 0 || directoryBirth.Inode == 0 {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	records["generation-directory.json"] = journal.RetainedRecord{Identity: r.reader.files[birthPath].identity, Bytes: body}
	directory := filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest)
	info, err := os.Lstat(directory)
	if err != nil || info == nil || !info.IsDir() {
		return nil, nil, errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || uint64(native.Dev) != directoryBirth.Device || native.Ino != directoryBirth.Inode || !((info.Mode() == os.ModeDir|0700 && (native.Gid == 0 || native.Gid == r.reader.gid)) || (info.Mode() == os.ModeDir|0750 && native.Gid == r.reader.gid)) {
		return nil, nil, ErrBinding
	}
	if original, known := r.reader.directories[directory]; known && !sameObservedDirectory(original, info) {
		return nil, nil, ErrBinding
	}
	r.reader.directories[directory] = info
	names := generation.Names()
	sort.Strings(names)
	images := make(map[string]generation.PrefixFile)
	inventory := make(map[string]bool)
	missing, incomplete := false, false
	for _, name := range names {
		recordName := generationFileRecordName(name)
		filename := filepath.Join(r.journal, recordName)
		raw, err := r.reader.read(ctx, filename, 4<<10, 0600, 0)
		if errors.Is(err, os.ErrNotExist) {
			missing = true
			continue
		}
		if err != nil || missing || incomplete {
			return nil, nil, errors.Join(ErrBinding, err)
		}
		var birth generationFileBirth
		mode := uint32(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		digest := r.intent.CandidateBinding.Files[name]
		if name == "binding.json" {
			digest = r.intent.Candidate.BindingDigest
		}
		if decodeCanonical(raw, 4<<10, &birth) != nil || birth.Schema != "ardents-endpoint-generation-file-v1" || birth.GenerationDigest != r.intent.Candidate.GenerationDigest || birth.Name != name || birth.Digest != digest || !canonicalDigest(digest) || birth.Size < 1 || birth.Size > 64<<20 || birth.Mode != mode || birth.GID != r.reader.gid || birth.Device == 0 || birth.Inode == 0 {
			return nil, nil, ErrBinding
		}
		if name == "binding.json" {
			encoded, err := canonicalJSON(r.intent.CandidateBinding)
			if err != nil || birth.Size != int64(len(encoded)) {
				return nil, nil, errors.Join(ErrBinding, err)
			}
		}
		leaf := filepath.Join(directory, name)
		current, err := os.Lstat(leaf)
		if err != nil || current == nil || !current.Mode().IsRegular() {
			return nil, nil, errors.Join(ErrBinding, err)
		}
		actual, ok := current.Sys().(*syscall.Stat_t)
		if !ok || actual.Uid != 0 || actual.Nlink != 1 || uint64(actual.Dev) != birth.Device || actual.Ino != birth.Inode || !((current.Mode() == 0600 && (actual.Gid == 0 || actual.Gid == r.reader.gid)) || (current.Mode() == os.FileMode(mode) && actual.Gid == r.reader.gid)) {
			return nil, nil, ErrBinding
		}
		bytesOnDisk, err := r.reader.readObserved(ctx, leaf, birth.Size, current.Mode(), actual.Gid, true)
		if err != nil || int64(len(bytesOnDisk)) == birth.Size && digestHex(bytesOnDisk) != birth.Digest {
			return nil, nil, errors.Join(ErrBinding, err)
		}
		complete := int64(len(bytesOnDisk)) == birth.Size && current.Mode() == os.FileMode(mode) && actual.Gid == r.reader.gid
		if current.Mode() != 0600 && !complete {
			return nil, nil, ErrBinding
		}
		incomplete = !complete
		r.writing.births[name] = birth
		images[name] = generation.PrefixFile{Identity: r.reader.files[leaf].identity, Bytes: bytesOnDisk}
		inventory[name], r.inventory[recordName] = true, true
		records[recordName] = journal.RetainedRecord{Identity: r.reader.files[filename].identity, Bytes: raw}
	}
	if (info.Mode() != os.ModeDir|0700 || native.Gid != 0) && (len(images) != len(names) || incomplete) {
		return nil, nil, ErrBinding
	}
	if err := recoveryInventory(directory, inventory); err != nil {
		return nil, nil, err
	}
	if err := recoveryInventory(r.journal, r.inventory); err != nil {
		return nil, nil, err
	}
	return records, images, ctx.Err()
}

func (r *successorRecoveryNative) completeWritingTransition(ctx context.Context, reference time.Time, verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs) (result ProvisionResult, returnedErr error) {
	owned := r.pending
	if owned == nil || owned.ctx != ctx || owned.stage == nil || owned.stage.prefix == nil || owned.authorization != nil || owned.predecessor != nil {
		return ProvisionResult{}, ErrBinding
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, ctx.Err())
		if owned.started != nil && owned.started.acknowledged() {
			result = ProvisionResult{Status: "installed-recovered-started", GenerationDigest: r.intent.Candidate.GenerationDigest}
		}
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			if owned.stage.intentAttempted {
				owned.stage.retainFailure(ctx, returnedErr)
			}
		}
	}()
	if err := owned.observe(); err != nil {
		return ProvisionResult{}, err
	}
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := successorContinuity(r.intent.CandidateBinding, floors, input.Local); err != nil {
		return ProvisionResult{}, err
	}
	authorization, err := generationauthorization.AuthenticateCandidate(ctx, verifier, candidate, input)
	if err != nil {
		return ProvisionResult{}, err
	}
	checked, err := recoverBoundGeneration(ctx, r.intent.Candidate, r.intent.CandidateBinding, r.intent.Request, reference, authorization)
	if err != nil {
		return ProvisionResult{}, err
	}
	full := make(map[string][]byte, len(generation.Names()))
	for name, body := range checked.files {
		full[name] = bytes.Clone(body)
	}
	full["binding.json"], err = canonicalJSON(checked.binding)
	if err != nil {
		return ProvisionResult{}, err
	}
	for name, birth := range r.writing.births {
		if birth.Size != int64(len(full[name])) || birth.Digest != digestHex(full[name]) {
			return ProvisionResult{}, ErrBinding
		}
	}
	if err := owned.stage.prefix.Match(full); err != nil {
		return ProvisionResult{}, err
	}
	paths := make([]string, 0)
	for filename := range r.reader.files {
		if filename == filepath.Join(r.reader.lease.path, "transition.json") || filepath.Dir(filename) == r.journal || filepath.Dir(filename) == filepath.Join(r.reader.lease.path, "preparation") {
			paths = append(paths, filename)
		}
	}
	sort.Strings(paths)
	for _, filename := range paths {
		if err := r.reader.syncObserved(ctx, filename); err != nil {
			return ProvisionResult{}, err
		}
	}
	if err := owned.observe(); err != nil {
		return ProvisionResult{}, err
	}
	owned.authorization = &authorization
	// Retain the actual original predecessor before admitted writing makes its
	// failure cleanup owe quiescence. This observation requests no Stop.
	if _, err := owned.retainPredecessor(); err != nil {
		return ProvisionResult{}, err
	}
	if r.writing.failedPhase {
		failed := owned.stage.journal.Bytes(journal.Transitions, "0002.json")
		if len(owned.stage.journal.Bytes(journal.Transitions, "original-transition-failure.json")) == 0 {
			if err := owned.stage.journal.Write(ctx, journal.Transitions, "original-transition-failure.json", failed); err != nil {
				return ProvisionResult{}, err
			}
		}
		if err := owned.observe(); err != nil {
			return ProvisionResult{}, err
		}
		if err := owned.stage.journal.RetireGenerationFailure(ctx); err != nil {
			return ProvisionResult{}, err
		}
		delete(r.reader.files, filepath.Join(r.journal, "0002.json"))
		delete(r.inventory, "0002.json")
		r.inventory["original-transition-failure.json"] = true
	}
	directory := filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest)
	// Transfer only these exact original leaf observations to Prefix's retained
	// descriptors. No generic cache refresh or creation-owner adoption occurs.
	for filename := range r.reader.files {
		if filepath.Dir(filename) == directory {
			delete(r.reader.files, filename)
		}
	}
	owned.stage.intentAttempted = true
	names := generation.Names()
	sort.Strings(names)
	for _, name := range names {
		recordName := generationFileRecordName(name)
		if _, known := r.writing.births[name]; known {
			if err := owned.stage.journal.Resync(ctx, journal.Transitions, recordName); err != nil {
				return ProvisionResult{}, err
			}
		} else {
			if err := owned.observe(); err != nil {
				return ProvisionResult{}, err
			}
			born, err := owned.stage.prefix.CreateFile(name)
			if err != nil {
				return ProvisionResult{}, err
			}
			native, ok := born.Sys().(*syscall.Stat_t)
			if !ok || born.Mode() != 0600 || born.Size() != 0 || native.Uid != 0 || native.Gid != 0 || native.Nlink != 1 {
				return ProvisionResult{}, ErrBinding
			}
			mode := uint32(0640)
			if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
				mode = 0555
			}
			record, err := canonicalJSON(generationFileBirth{Schema: "ardents-endpoint-generation-file-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, Name: name, Device: uint64(native.Dev), Inode: native.Ino, Digest: digestHex(full[name]), Size: int64(len(full[name])), Mode: mode, GID: r.reader.gid})
			if err != nil {
				return ProvisionResult{}, err
			}
			if err := owned.stage.journal.Write(ctx, journal.Transitions, recordName, record); err != nil {
				return ProvisionResult{}, err
			}
		}
		if err := owned.observe(); err != nil {
			return ProvisionResult{}, err
		}
		if err := owned.stage.prefix.Repair(name, full[name]); err != nil {
			return ProvisionResult{}, err
		}
	}
	if err := owned.stage.prefix.Seal(); err != nil {
		return ProvisionResult{}, err
	}
	promoted, err := os.Lstat(directory)
	identity := owned.stage.prefix.Identity()
	if err != nil || promoted == nil {
		return ProvisionResult{}, errors.Join(ErrBinding, err)
	}
	native, ok := promoted.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || uint64(native.Dev) != identity.Device || native.Ino != identity.Inode || native.Gid != identity.GID || promoted.Mode() != identity.Mode {
		return ProvisionResult{}, ErrBinding
	}
	r.reader.directories[directory] = promoted // Exact admitted same-inode seal.
	owned.stage.sealedIdentity = promoted
	parent := filepath.Dir(directory)
	owned.stage.sealed, err = generation.OpenSnapshot(ctx, parent, r.reader.directories[parent], promoted, r.intent.Candidate.GenerationDigest, r.reader.gid)
	if err != nil {
		return ProvisionResult{}, err
	}
	binding, actualFiles, err := r.reader.retainStagedGeneration(ctx, owned.stage)
	if err != nil || !bytes.Equal(binding, full["binding.json"]) {
		return ProvisionResult{}, errors.Join(ErrBinding, err)
	}
	selectedBody, err := canonicalJSON(r.intent.Candidate)
	if err != nil {
		return ProvisionResult{}, err
	}
	actual, err := inspectGeneration(r.reader.lease.path, selectedBody, binding, actualFiles)
	if err != nil {
		return ProvisionResult{}, err
	}
	r.checked = actual
	if err := owned.stage.record(ctx, "generation-staged", nil); err != nil {
		return ProvisionResult{}, err
	}
	r.inventory["0002.json"] = true
	r.writing = nil
	delete(r.reader.files, filepath.Join(r.reader.lease.path, "transition.json"))
	if err := owned.predecessor.stopAndJoin(); err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{}, owned.startCandidate()
}

// Admit a complete sealed candidate before selection publication.
// The prefix may include the replacement intention and its exact record group.
// Other prefixes remain repair-required. All retained owners are independent
// observations, not fabricated creation custody or successful manager facts.
func (r *successorRecoveryNative) retainStagedTransition(ctx context.Context) error {
	for _, name := range []string{"start-guard.json", "start-completion.socket", "start-socket.json"} {
		if _, err := r.reader.lease.root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	records, err := r.readStagedJournal(ctx)
	if err != nil {
		return err
	}
	if err := r.checkStagedSelection(); err != nil {
		return err
	}
	if err := r.checkStagedFixedResources(); err != nil {
		return err
	}
	directory := filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest)
	info := r.reader.directories[directory]
	requestBody, err := canonicalJSON(r.intent.Request)
	if err != nil {
		return err
	}
	request, err := DecodeRequest(ctx, requestBody, false)
	if err != nil {
		return err
	}
	stage := &installationTransaction{lease: r.reader.lease, selected: r.intent.Candidate, intent: r.reader.files[filepath.Join(r.reader.lease.path, "transition.json")], fixed: make(map[string]fixedFileObservation), sealedIdentity: info}
	owned := &successorPreparation{ctx: ctx, request: request, inspection: r.reader, previous: r.previous, stage: stage, recovery: r}
	r.pending = owned // Register partial handles before any subsequent open fails.
	parent := filepath.Dir(directory)
	stage.sealed, err = generation.OpenSnapshot(ctx, parent, r.reader.directories[parent], info, r.intent.Candidate.GenerationDigest, r.reader.gid)
	if err != nil {
		return err
	}
	var replacements []journal.RetainedReplacements
	if r.stagedReplacements != nil {
		replacements = append(replacements, *r.stagedReplacements)
	}
	stage.journal, err = journal.OpenTransition(ctx, filepath.Dir(r.journal), r.reader.directories[filepath.Dir(r.journal)], r.reader.directories[r.journal], r.intent.Candidate.GenerationDigest, records, replacements...)
	if err != nil {
		return err
	}
	selection := filepath.Join(r.reader.lease.path, "selection.json")
	stage.fixed[selection] = fixedFileObservation{parent: r.reader.lease.identity, file: r.reader.files[selection]}
	if err := owned.observe(); err != nil {
		return err
	}
	if r.fixedMutation {
		// Observe actual stopped units and empty scopes before even opening the
		// Release composition. This owner requests no Stop or other effect.
		_, err := owned.retainPredecessor()
		return err
	}
	return nil
}

func (r *successorRecoveryNative) readStagedJournal(ctx context.Context) (map[string]journal.RetainedRecord, error) {
	r.inventory = map[string]bool{"0001.json": true, "0002.json": true, "generation-directory.json": true}
	r.stagedReplacements = nil
	phases := []struct{ name, phase string }{{"0001.json", "writing-generation"}, {"0002.json", "generation-staged"}}
	// A crash may follow the replacement intention before its first record-group
	// birth or while recording preimages. Partial fixed bytes require the complete
	// group and ordered same-inode copy grammar. Completed replacement requires
	// exact candidate files; publishing-selection permits only its recorded
	// original selection prefix. Reload prefixes require full selection provenance;
	// start phases and foreign groups refuse.
	if _, err := os.Lstat(filepath.Join(r.journal, "0003.json")); err == nil {
		r.inventory["0003.json"] = true
		phases = append(phases, struct{ name, phase string }{"0003.json", "replacing-fixed-resources"})
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	if _, err := os.Lstat(filepath.Join(r.journal, "0004.json")); err == nil {
		if !r.inventory["0003.json"] {
			return nil, ErrBinding
		}
		r.inventory["0004.json"] = true
		phases = append(phases, struct{ name, phase string }{"0004.json", "fixed-resources-replaced"})
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	if _, err := os.Lstat(filepath.Join(r.journal, "0005.json")); err == nil {
		if !r.inventory["0004.json"] {
			return nil, ErrBinding
		}
		r.inventory["0005.json"] = true
		phases = append(phases, struct{ name, phase string }{"0005.json", "publishing-selection"})
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	for _, item := range []struct{ name, previous, phase string }{{"0006.json", "0005.json", "reloading-manager"}, {"0007.json", "0006.json", "successor-reloaded-stopped"}} {
		if _, err := os.Lstat(filepath.Join(r.journal, item.name)); err == nil {
			if !r.inventory[item.previous] {
				return nil, ErrBinding
			}
			r.inventory[item.name] = true
			phases = append(phases, struct{ name, phase string }{item.name, item.phase})
		} else if !errors.Is(err, os.ErrNotExist) {
			return nil, errors.Join(ErrBinding, err)
		}
	}
	groupPath := filepath.Join(r.journal, "replacements")
	groupIdentity, groupErr := os.Lstat(groupPath)
	if groupErr == nil {
		if !r.inventory["0003.json"] || !privateJournalDirectory(groupIdentity) {
			return nil, ErrBinding
		}
		r.inventory["replacements"] = true
	} else if !errors.Is(groupErr, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, groupErr)
	}
	birthRecords, err := readGenerationFileBirths(ctx, r.reader, r.journal, r.intent.Candidate, r.checked.binding, r.checked.files, r.inventory)
	if err != nil {
		return nil, err
	}
	if err := r.readGenerationFailure(ctx, birthRecords); err != nil {
		return nil, err
	}
	if err := recoveryInventory(r.journal, r.inventory); err != nil {
		return nil, err
	}
	records := birthRecords
	for _, item := range phases {
		filename := filepath.Join(r.journal, item.name)
		body, err := r.reader.read(ctx, filename, 64<<10, 0600, 0)
		var observed generationTransition
		if err != nil || decodeCanonical(body, 64<<10, &observed) != nil || observed != (generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, BindingDigest: r.intent.Candidate.BindingDigest, Phase: item.phase}) {
			return nil, errors.Join(ErrBinding, err)
		}
		records[item.name] = journal.RetainedRecord{Identity: r.reader.files[filename].identity, Bytes: body}
	}
	birthPath := filepath.Join(r.journal, "generation-directory.json")
	body, err := r.reader.read(ctx, birthPath, 4<<10, 0600, 0)
	var birth generationBirth
	directory := filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest)
	info := r.reader.directories[directory]
	if err != nil || info == nil || decodeCanonical(body, 4<<10, &birth) != nil || birth.Schema != "ardents-endpoint-generation-directory-v1" || birth.GenerationDigest != r.intent.Candidate.GenerationDigest {
		return nil, errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || birth.Device == 0 || birth.Inode == 0 || birth.Device != uint64(native.Dev) || birth.Inode != native.Ino {
		return nil, ErrBinding
	}
	records["generation-directory.json"] = journal.RetainedRecord{Identity: r.reader.files[birthPath].identity, Bytes: body}
	if groupErr == nil {
		if err := r.readStagedReplacements(ctx, groupIdentity); err != nil {
			return nil, err
		}
	}
	return records, ctx.Err()
}

// Retain the actual current bytes, including an empty interrupted copy. Complete
// preimages come separately from the sealed predecessor, never this read cache.
func (r *successorRecoveryNative) readStagedFixedResources(ctx context.Context) error {
	resources, err := recoveryFixedBytes(r.previous)
	if err != nil {
		return err
	}
	delete(resources, filepath.Join(r.reader.lease.path, "selection.json"))
	for filename := range resources {
		mode := os.FileMode(0644)
		if filename == "/usr/lib/ardents/text-worker-root/ardents-text" {
			mode = 0555
		}
		if _, err := r.reader.readObserved(ctx, filename, 64<<20, mode, 0, true); err != nil {
			return err
		}
	}
	return ctx.Err()
}

// Pending selection is an observed image, never a trusted generation pointer.
// Only the publishing intention and exact original record permit torn bytes.
func (r *successorRecoveryNative) checkStagedSelection() error {
	filename := filepath.Join(r.reader.lease.path, "selection.json")
	current, known := r.reader.files[filename]
	if !known || current.identity == nil || current.mode != 0640 || current.gid != r.reader.gid {
		return ErrBinding
	}
	before, err := canonicalJSON(r.intent.Previous)
	if err != nil {
		return err
	}
	if !r.inventory["0005.json"] {
		if !bytes.Equal(current.body, before) {
			return ErrBinding
		}
		return nil
	}
	after, err := canonicalJSON(r.intent.Candidate)
	if err != nil {
		return err
	}
	if r.inventory["0006.json"] && !bytes.Equal(current.body, after) {
		return ErrBinding
	}
	if !fixedfile.ReplacementPrefixAllowed(current.body, before, after) {
		return ErrBinding
	}
	if (!bytes.Equal(current.body, before) || r.inventory["0006.json"]) && (r.stagedReplacements == nil || r.stagedReplacements.Records[digestHex([]byte(filename))+".json"].Identity == nil) {
		return ErrBinding
	}
	return nil
}

// All nine intentions precede every write. Actual bytes must follow the same
// sorted copy order: complete candidate images, at most one torn image, then
// complete predecessor images. Unchanged old/new images are order-neutral.
func (r *successorRecoveryNative) checkStagedFixedResources() error {
	before, err := recoveryFixedBytes(r.previous)
	if err != nil {
		return err
	}
	after, err := recoveryFixedBytes(r.checked)
	if err != nil {
		return err
	}
	delete(before, filepath.Join(r.reader.lease.path, "selection.json"))
	paths := make([]string, 0, len(before))
	for filename := range before {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	completed := r.inventory["0004.json"]
	boundary, mutated := false, false
	for _, filename := range paths {
		current, known := r.reader.files[filename]
		if !known || current.identity == nil || !fixedfile.ReplacementPrefixAllowed(current.body, before[filename], after[filename]) {
			return ErrBinding
		}
		old, candidate := bytes.Equal(current.body, before[filename]), bytes.Equal(current.body, after[filename])
		if completed && !candidate {
			return ErrBinding
		}
		if boundary && !old {
			return ErrBinding
		}
		if !candidate {
			boundary = true
		}
		mutated = mutated || !old
	}
	want := len(paths)
	if r.stagedReplacements != nil && r.stagedReplacements.Records[digestHex([]byte(filepath.Join(r.reader.lease.path, "selection.json")))+".json"].Identity != nil {
		want++
	}
	if (mutated || completed) && (r.stagedReplacements == nil || len(r.stagedReplacements.Records) != want) {
		return ErrBinding
	}
	// The completion record also requires continuing quiescence when all static
	// bytes happen to be identical between the two generations.
	r.fixedMutation = mutated || completed
	return nil
}

// Replacement provenance is written in path order before the first mutation.
// Admit that prefix against complete sealed preimages and actual fixed inodes.
// The selection record follows all nine fixed records only during publication;
// later manager phases remain outside this admission.
func (r *successorRecoveryNative) readStagedReplacements(ctx context.Context, identity os.FileInfo) error {
	resources, err := recoveryFixedBytes(r.checked)
	if err != nil {
		return err
	}
	selection := filepath.Join(r.reader.lease.path, "selection.json")
	delete(resources, selection)
	preimages, err := recoveryFixedBytes(r.previous)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(resources))
	for filename := range resources {
		paths = append(paths, filename)
	}
	sort.Strings(paths)
	if r.inventory["0005.json"] {
		resources[selection], err = canonicalJSON(r.intent.Candidate)
		if err != nil {
			return err
		}
		preimages[selection], err = canonicalJSON(r.intent.Previous)
		if err != nil {
			return err
		}
		// Selection provenance follows all nine fixed records, not path sorting.
		paths = append(paths, selection)
	}
	directory := filepath.Join(r.journal, "replacements")
	if original, known := r.reader.directories[directory]; known && !sameObservedDirectory(original, identity) {
		return ErrBinding
	}
	r.reader.directories[directory] = identity
	allowed := make(map[string]bool, len(paths))
	records := make(map[string]journal.RetainedRecord)
	missing := false
	for _, filename := range paths {
		name := digestHex([]byte(filename)) + ".json"
		allowed[name] = false
		if _, err := os.Lstat(filepath.Join(directory, name)); errors.Is(err, os.ErrNotExist) {
			missing = true
			continue
		} else if err != nil {
			return errors.Join(ErrBinding, err)
		}
		if missing {
			return ErrBinding
		}
		body, err := r.reader.read(ctx, filepath.Join(directory, name), 4<<10, 0600, 0)
		observed := r.reader.files[filename]
		var record fixedReplacementRecord
		if err != nil || observed.identity == nil || len(observed.body) > 64<<20 || decodeCanonical(body, 4<<10, &record) != nil {
			return errors.Join(ErrBinding, err)
		}
		native, ok := observed.identity.Sys().(*syscall.Stat_t)
		if !ok || native.Dev == 0 || native.Ino == 0 || native.Uid != 0 || native.Gid != observed.gid || native.Nlink != 1 ||
			record != (fixedReplacementRecord{Schema: "ardents-endpoint-installation-replacement-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, Path: filename, Device: uint64(native.Dev), Inode: native.Ino, Mode: uint32(observed.mode), GID: observed.gid, PreviousDigest: digestHex(preimages[filename]), CandidateDigest: digestHex(resources[filename])}) {
			return ErrBinding
		}
		allowed[name] = true
		records[name] = journal.RetainedRecord{Identity: r.reader.files[filepath.Join(directory, name)].identity, Bytes: body}
	}
	if err := recoveryInventory(directory, allowed); err != nil {
		return err
	}
	r.stagedReplacements = &journal.RetainedReplacements{Identity: identity, Records: records}
	return ctx.Err()
}

func (r *successorRecoveryNative) completeStagedTransition(ctx context.Context, reference time.Time, verifier *release.Verifier, candidate enrollment.Candidate, input release.Inputs) (result ProvisionResult, returnedErr error) {
	owned := r.pending
	if owned == nil || owned.ctx != ctx || owned.authorization != nil || owned.predecessor != nil && !r.fixedMutation {
		return ProvisionResult{}, ErrInput
	}
	if r.fixedMutation && (owned.predecessor == nil || !owned.predecessor.alreadyStopped || !owned.predecessor.joined) {
		return ProvisionResult{}, ErrBinding
	}
	defer func() {
		returnedErr = errors.Join(returnedErr, ctx.Err())
		if owned.started != nil && owned.started.acknowledged() {
			result = ProvisionResult{Status: "installed-recovered-started", GenerationDigest: r.intent.Candidate.GenerationDigest}
		}
		if returnedErr != nil {
			owned.terminal = errors.Join(owned.terminal, returnedErr)
			// Before recovery admission no transaction record is changed.
			if owned.stage.intentAttempted {
				owned.stage.retainFailure(ctx, returnedErr)
			}
		}
	}()
	if err := owned.observe(); err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	if r.fixedMutation {
		if err := owned.predecessor.observe(); err != nil {
			return ProvisionResult{}, err
		}
	}
	floors, err := verifier.CurrentFloors(ctx)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := successorContinuity(r.checked.binding, floors, input.Local); err != nil {
		return ProvisionResult{}, err
	}
	authorization, err := generationauthorization.AuthenticateCandidate(ctx, verifier, candidate, input)
	if err != nil {
		return ProvisionResult{}, err
	}
	checked, err := recoverBoundGeneration(ctx, r.intent.Candidate, r.intent.CandidateBinding, r.intent.Request, reference, authorization)
	if err != nil {
		return ProvisionResult{}, errors.Join(ErrRepairRequired, err)
	}
	for name, body := range checked.files {
		if !bytes.Equal(body, r.checked.files[name]) || !bytes.Equal(body, owned.stage.generationBytes(name)) {
			return ProvisionResult{}, errors.Join(ErrRepairRequired, ErrBinding)
		}
	}
	if err := owned.observe(); err != nil {
		return ProvisionResult{}, err
	}
	// Visible original records and complete candidate bytes are resynchronized
	// before this recovery may stop or mutate anything. No earlier ACK is replayed.
	paths := make([]string, 0)
	for filename := range r.reader.files {
		if filename == filepath.Join(r.reader.lease.path, "transition.json") || filepath.Dir(filename) == r.journal || filepath.Dir(filename) == filepath.Join(r.journal, "replacements") || filepath.Dir(filename) == filepath.Join(r.reader.lease.path, "generations", r.intent.Candidate.GenerationDigest) {
			paths = append(paths, filename)
		}
	}
	sort.Strings(paths)
	for _, filename := range paths {
		if err := r.reader.syncObserved(ctx, filename); err != nil {
			return ProvisionResult{}, err
		}
	}
	if r.stagedReplacements != nil {
		if err := owned.stage.journal.ResyncReplacements(ctx); err != nil {
			return ProvisionResult{}, err
		}
	}
	if err := owned.observe(); err != nil {
		return ProvisionResult{}, err
	}
	owned.authorization = &authorization
	if owned.predecessor == nil {
		if _, err := owned.retainPredecessor(); err != nil {
			return ProvisionResult{}, err
		}
	} else if err := owned.predecessor.observe(); err != nil {
		return ProvisionResult{}, err
	}
	// Only now does failure cleanup owe original predecessor quiescence. The
	// stage retains intent custody through archival; avoid a second read cache
	// requiring that same file to remain present after its admitted removal.
	owned.stage.intentAttempted = true
	delete(r.reader.files, filepath.Join(r.reader.lease.path, "transition.json"))
	if r.inventory["0006.json"] {
		if err := owned.resumeReload(); err != nil {
			return ProvisionResult{}, err
		}
		return ProvisionResult{}, owned.startCandidate()
	}
	if err := owned.predecessor.stopAndJoin(); err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{}, owned.startCandidate()
}

// This terminal cleanup boundary requires all three original removal records.
// Their presence is not ACK evidence. Fresh proofs, selected generation, exact
// replacements and the still-running original invocation are separate checks.
func (r *successorRecoveryNative) readRemovals(ctx context.Context) error {
	for index, name := range []string{"completion-socket-removal.json", "completion-record-removal.json", "completion-guard-removal.json"} {
		body, err := r.reader.read(ctx, filepath.Join(r.journal, name), 4<<10, 0600, 0)
		if err != nil || decodeCanonical(body, 4<<10, &r.removals[index]) != nil {
			return errors.Join(ErrBinding, err)
		}
	}
	if err := validateCompletionRemovals(r.intentBody, r.reader.gid, r.removals); err != nil {
		return err
	}
	// Older producers recorded only the observed invocation. Preserve that
	// compatibility; an optional new intention must bind the same original
	// socket birth and intent, and grants no Start or ACK authority on reopen.
	filename := filepath.Join(r.journal, "start-attempt.json")
	body, err := r.reader.read(ctx, filename, 4<<10, 0600, 0)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if errors.Is(err, os.ErrNotExist) && r.reader.files[filename].identity != nil {
		return ErrBinding
	}
	if r.inventory != nil {
		if err == nil {
			r.inventory["start-attempt.json"] = true
		} else {
			// Historical absence is fixed for THIS opening. A late optional
			// record cannot enter the admitted inventory without observation.
			delete(r.inventory, "start-attempt.json")
		}
	}
	if err == nil {
		socketBody, err := canonicalJSON(completionSocketBirth{Schema: "ardents-endpoint-installation-start-socket-v1", IntentDigest: digestHex(r.intentBody), Device: r.removals[0].Device, Inode: r.removals[0].Inode, GID: r.reader.gid})
		var attempt candidateStartAttempt
		if err != nil || decodeCanonical(body, 4<<10, &attempt) != nil || attempt != (candidateStartAttempt{Schema: "ardents-endpoint-installation-start-attempt-v1", IntentDigest: digestHex(r.intentBody), SocketRecordDigest: digestHex(socketBody)}) {
			return errors.Join(ErrBinding, err)
		}
	}
	return r.observeRemovals(ctx)
}

func validateCompletionRemovals(intent []byte, gid uint32, records [3]completionRemoval) error {
	names := []string{"start-completion.socket", "start-socket.json", "start-guard.json"}
	socketBody, err := canonicalJSON(completionSocketBirth{Schema: "ardents-endpoint-installation-start-socket-v1", IntentDigest: digestHex(intent), Device: records[0].Device, Inode: records[0].Inode, GID: gid})
	if err != nil || len(intent) == 0 || gid == 0 {
		return errors.Join(ErrBinding, err)
	}
	for index, record := range records {
		mode, group, size, digest := uint32(syscall.S_IFREG|0600), uint32(0), int64(len(intent)), digestHex(intent)
		if index == 0 {
			mode, group, size, digest = syscall.S_IFSOCK|0660, gid, 0, ""
		}
		if index == 1 {
			size, digest = int64(len(socketBody)), digestHex(socketBody)
		}
		if record.Schema != "ardents-endpoint-installation-completion-removal-v1" || record.IntentDigest != digestHex(intent) || record.Name != names[index] || record.Device == 0 || record.Inode == 0 || record.UID != 0 || record.GID != group || record.Mode != mode || record.Size != size || record.BytesDigest != digest || record.ChangedNSec < 0 || record.ChangedNSec >= 1_000_000_000 || record.ModifiedNSec < 0 || record.ModifiedNSec >= 1_000_000_000 {
			return ErrBinding
		}
	}
	// The two original regular records cannot designate the same physical file.
	if records[1].Device == records[2].Device && records[1].Inode == records[2].Inode {
		return ErrBinding
	}
	return nil
}

func (r *successorRecoveryNative) observeRemovals(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := r.reader.lease.observe(); err != nil {
		return err
	}
	// Reaching the guard removal record requires the original producer to have
	// removed socket and socket-record first. Never adopt an out-of-order prefix.
	for _, name := range []string{"start-completion.socket", "start-socket.json"} {
		if _, err := r.reader.lease.root.Lstat(name); !errors.Is(err, os.ErrNotExist) {
			return errors.Join(ErrBinding, err)
		}
	}
	info, err := r.reader.lease.root.Lstat("start-guard.json")
	if errors.Is(err, os.ErrNotExist) {
		// An object retained by THIS opening disappearing is substitution/loss,
		// not permission to forget custody or infer physical completion.
		if r.reader.files[filepath.Join(r.reader.lease.path, "start-guard.json")].identity != nil {
			return ErrBinding
		}
		return errors.Join(r.reader.lease.observe(), ctx.Err())
	}
	if err != nil || !completionRemovalMatches(r.removals[2], info) {
		return errors.Join(ErrBinding, err)
	}
	body, err := r.reader.read(ctx, filepath.Join(r.reader.lease.path, "start-guard.json"), 128<<10, 0600, 0)
	if err != nil || !bytes.Equal(body, r.intentBody) {
		return errors.Join(ErrBinding, err)
	}
	info, err = r.reader.lease.root.Lstat("start-guard.json")
	if err != nil || !completionRemovalMatches(r.removals[2], info) {
		return errors.Join(ErrBinding, err)
	}
	return errors.Join(r.reader.lease.observe(), ctx.Err())
}

func completionRemovalMatches(record completionRemoval, info os.FileInfo) bool {
	if info == nil || !info.Mode().IsRegular() {
		return false
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	return ok && native.Nlink == 1 && record.Device == uint64(native.Dev) && record.Inode == native.Ino && record.UID == native.Uid && record.GID == native.Gid && record.Mode == native.Mode && record.Size == native.Size && record.ChangedSec == native.Ctim.Sec && record.ChangedNSec == native.Ctim.Nsec && record.ModifiedSec == native.Mtim.Sec && record.ModifiedNSec == native.Mtim.Nsec
}

func (r *successorRecoveryNative) finishRemoval(ctx context.Context) error {
	// Sync the exact private intention before touching its original inode. The
	// post-I/O observation checks timestamps as well as bytes and identity.
	if err := r.reader.syncObserved(ctx, filepath.Join(r.journal, "completion-guard-removal.json")); err != nil {
		return err
	}
	if err := r.observeRemovals(ctx); err != nil {
		return err
	}
	filename := filepath.Join(r.reader.lease.path, "start-guard.json")
	if r.reader.files[filename].identity != nil {
		if err := r.reader.removeObserved(ctx, filename); err != nil {
			return err
		}
	}
	// Absence observed on reopen is not a receipt for the former parent's sync.
	return errors.Join(syncStagingRoot(r.reader.lease.root), r.observeRemovals(ctx), ctx.Err())
}
