package endpoint

import (
	"bytes"
	"context"
	"errors"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/cgroup"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/journal"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/systemd"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type recoveryNative struct {
	reader      *installedRoot
	intent      initialTransitionIntent
	journal     string
	generation  string
	creations   map[string]fixedCreationRecord
	access      map[string]fixedDirectoryCreation
	inventories map[string]map[string]bool
}

func openInitialRecovery(ctx context.Context, root string, reference time.Time) (result *Recovery, returnedErr error) {
	if err := observeInstallationPlatform(ctx); err != nil {
		return nil, err
	}
	reader, err := openInstalledRoot(ctx, root)
	if err != nil {
		return nil, errors.Join(ErrRepairRequired, err)
	}
	native := &recoveryNative{reader: reader, creations: make(map[string]fixedCreationRecord), access: make(map[string]fixedDirectoryCreation), inventories: make(map[string]map[string]bool)}
	defer func() {
		if returnedErr != nil {
			returnedErr = errors.Join(ErrRepairRequired, returnedErr, closeInitialRecovery(native))
		}
	}()
	if _, err := reader.lease.root.Lstat("start-guard.json"); !errors.Is(err, os.ErrNotExist) {
		return nil, errors.Join(ErrBinding, err)
	}
	body, err := readInitialRecoveryIntent(ctx, reader)
	if err != nil || decodeCanonical(body, 128<<10, &native.intent) != nil {
		return nil, errors.Join(ErrBinding, err)
	}
	intent := native.intent
	bindingBody, err := canonicalJSON(intent.CandidateBinding)
	requestBody, requestErr := canonicalJSON(intent.Request)
	if err != nil || requestErr != nil || intent.Schema != "ardents-endpoint-installation-initial-v1" || intent.Previous != (generationSelection{}) ||
		intent.Candidate.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(intent.Candidate.GenerationDigest) || digestHex(bindingBody) != intent.Candidate.BindingDigest ||
		intent.CandidateBinding.Schema != "ardents-endpoint-installation-binding-v1" || intent.CandidateBinding.InstallationRoot != root || intent.CandidateBinding.GenerationDigest != intent.Candidate.GenerationDigest ||
		intent.CandidateBinding.Account != "ardents-endpoint" || intent.CandidateBinding.Unit != "ardents-endpoint.service" || intent.CandidateBinding.GID != reader.gid || len(intent.CandidateBinding.Files) != 14 ||
		intent.Request.InstallationRoot != root || intent.Request.ReleaseFloorRoot != intent.CandidateBinding.ReleaseFloorRoot {
		return nil, errors.Join(ErrBinding, err, requestErr)
	}
	if _, err := DecodeRequest(ctx, requestBody, true); err != nil {
		return nil, err
	}
	originalTime, err := time.Parse(time.RFC3339Nano, intent.Request.ReferenceTime)
	if err != nil || reference.Before(originalTime) {
		return nil, ErrBinding
	}
	if _, err := rootDirectoryAncestors(intent.Request.BundleRoot); err != nil {
		return nil, err
	}
	if _, err := rootDirectoryAncestors(intent.Request.ReleaseFloorRoot); err != nil {
		return nil, err
	}
	wantedRoots := mutableRoots(intent.Request.Headless)
	if len(wantedRoots) != len(intent.CandidateBinding.MutableRoots) {
		return nil, ErrBinding
	}
	for index, identity := range intent.CandidateBinding.MutableRoots {
		if identity.Path != wantedRoots[index] || identity.Device == 0 || identity.Inode == 0 {
			return nil, ErrBinding
		}
	}
	checked := inspectedGeneration{binding: intent.CandidateBinding, request: intent.Request}
	if err := reader.observeAccountAndRoots(checked); err != nil {
		return nil, err
	}
	if err := native.readPreparation(ctx, digestHex(requestBody)); err != nil {
		return nil, err
	}
	native.journal = filepath.Join(root, "journals", intent.Candidate.GenerationDigest)
	native.generation = filepath.Join(root, "generations", intent.Candidate.GenerationDigest)
	native.inventories[root] = map[string]bool{"writer.lock": true, "preparation": true, "journals": true, "generations": true, "selection.json": true, "transition.json": false}
	native.inventories[filepath.Join(root, "generations")] = map[string]bool{intent.Candidate.GenerationDigest: true}
	native.inventories[filepath.Join(root, "journals")] = map[string]bool{intent.Candidate.GenerationDigest: true}
	for _, directory := range []string{filepath.Join(root, "generations"), native.generation} {
		if err := reader.pinGenerationDirectory(directory); err != nil {
			return nil, err
		}
	}
	if err := native.readJournal(ctx); err != nil {
		return nil, err
	}
	if err := native.readDirectories(ctx); err != nil {
		return nil, err
	}
	if err := native.readFixedCreations(ctx); err != nil {
		return nil, err
	}
	names := make(map[string]bool, 15)
	for name := range intent.CandidateBinding.Files {
		if filepath.Base(name) != name || name == "." || name == ".." || name == "binding.json" || !canonicalDigest(intent.CandidateBinding.Files[name]) {
			return nil, ErrBinding
		}
		names[name] = true
	}
	names["binding.json"] = true
	if err := recoveryInventory(native.generation, names); err != nil {
		return nil, err
	}
	native.inventories[native.generation] = names
	for name := range names {
		mode := os.FileMode(0640)
		if name == "ardents-linux-amd64" || name == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if err := native.readPrefix(ctx, filepath.Join(native.generation, name), mode, reader.gid, nil); err != nil {
			return nil, err
		}
	}
	if err := native.observe(ctx); err != nil {
		return nil, err
	}
	return &Recovery{state: &recoveryOperation{ctx: ctx, native: native, request: intent.Request, reference: reference}}, nil
}

// A missing pending cursor permits only inspection of the exact selected
// archive. The reader retains joined filesystem errors, including ErrBinding;
// classify absence through that error tree rather than discarding its causes.
func readInitialRecoveryIntent(ctx context.Context, reader *installedRoot) ([]byte, error) {
	root := reader.lease.path
	body, err := reader.read(ctx, filepath.Join(root, "transition.json"), 128<<10, 0600, 0)
	if !errors.Is(err, os.ErrNotExist) {
		return body, err
	}
	selectedBody, selectionErr := reader.read(ctx, filepath.Join(root, "selection.json"), 4<<10, 0640, reader.gid)
	var selected generationSelection
	if selectionErr != nil || decodeCanonical(selectedBody, 4<<10, &selected) != nil || selected.Schema != "ardents-endpoint-installation-selection-v1" || !canonicalDigest(selected.GenerationDigest) || !canonicalDigest(selected.BindingDigest) {
		return nil, errors.Join(ErrBinding, selectionErr)
	}
	body, err = reader.read(ctx, filepath.Join(root, "journals", selected.GenerationDigest, "completed-intent.json"), 128<<10, 0600, 0)
	if err != nil {
		return nil, err
	}
	var archived initialTransitionIntent
	if decodeCanonical(body, 128<<10, &archived) != nil || archived.Candidate != selected {
		return nil, ErrBinding
	}
	return body, nil
}

func (r *recoveryNative) readPreparation(ctx context.Context, requestDigest string) error {
	directory := filepath.Join(r.reader.lease.path, "preparation")
	allowed := map[string]bool{"0001.json": true, "0002.json": true, "0003.json": true, "failure.json": false}
	if err := recoveryInventory(directory, allowed); err != nil {
		return err
	}
	r.inventories[directory] = allowed
	var previous journal.Record
	for _, name := range []string{"0001.json", "0002.json", "0003.json", "failure.json"} {
		body, err := r.reader.read(ctx, filepath.Join(directory, name), 64<<10, 0600, 0)
		if name == "failure.json" && errors.Is(err, os.ErrNotExist) {
			continue
		}
		var current journal.Record
		if err != nil || decodeCanonical(body, 64<<10, &current) != nil || current.RequestDigest != requestDigest || current.GenerationDigest != r.intent.Candidate.GenerationDigest {
			return errors.Join(ErrBinding, err)
		}
		next, err := journal.Next(previous, current)
		if err != nil || next != name {
			return errors.Join(ErrBinding, err)
		}
		if name != "0001.json" && (current.UID != r.intent.CandidateBinding.UID || current.GID != r.intent.CandidateBinding.GID) {
			return ErrBinding
		}
		previous = current
	}
	return nil
}

func (r *recoveryNative) readJournal(ctx context.Context) error {
	phases := map[string]string{"0001.json": "writing-generation", "0002.json": "generation-staged", "0003.json": "installing-fixed-resources", "0004.json": "fixed-resources-installed", "0005.json": "publishing-selection", "0006.json": "reloading-manager", "0007.json": "installed-stopped",
		"original-transition-failure.json": "successor-transition-failed", "recovery-failure.json": "initial-recovery-failed", "initial-recovered.json": "initial-recovered-stopped",
		"original-transition-failure-archived.json": "successor-transition-failed", "recovery-failure-archived.json": "initial-recovery-failed"}
	allowed := map[string]bool{"generation-directory.json": true, "creations": true, "directory-creations": true, "completed-intent.json": false,
		"original-preparation-failure-archived.json": false, "original-transition-failure-archived.json": false, "recovery-failure-archived.json": false}
	for name := range phases {
		allowed[name] = name <= "0005.json"
	}
	if _, err := readGenerationFileBirths(ctx, r.reader, r.journal, r.intent.Candidate, r.intent.CandidateBinding, nil, allowed); err != nil {
		return err
	}
	if err := recoveryInventory(r.journal, allowed); err != nil {
		return err
	}
	r.inventories[r.journal] = allowed
	for name, phase := range phases {
		body, err := r.reader.read(ctx, filepath.Join(r.journal, name), 64<<10, 0600, 0)
		if errors.Is(err, os.ErrNotExist) && !allowed[name] {
			continue
		}
		var record generationTransition
		failure := phase == "successor-transition-failed" || phase == "initial-recovery-failed"
		if err != nil || decodeCanonical(body, 64<<10, &record) != nil || record.Schema != "ardents-endpoint-installation-transition-v1" || record.GenerationDigest != r.intent.Candidate.GenerationDigest || record.BindingDigest != r.intent.Candidate.BindingDigest || record.Phase != phase || failure != (record.OriginalError != "") {
			return errors.Join(ErrBinding, err)
		}
	}
	if _, completed := r.reader.files[filepath.Join(r.journal, "0007.json")]; completed {
		if _, reloaded := r.reader.files[filepath.Join(r.journal, "0006.json")]; !reloaded {
			return ErrBinding
		}
	}
	if body, err := r.reader.read(ctx, filepath.Join(r.journal, "original-preparation-failure-archived.json"), 64<<10, 0600, 0); !errors.Is(err, os.ErrNotExist) {
		var record journal.Record
		requestBody, requestErr := canonicalJSON(r.intent.Request)
		if err != nil || requestErr != nil || decodeCanonical(body, 64<<10, &record) != nil || record.Phase != "preparation-failed" || record.OriginalError == "" || record.GenerationDigest != r.intent.Candidate.GenerationDigest || record.RequestDigest != digestHex(requestBody) || record.UID != r.intent.CandidateBinding.UID || record.GID != r.intent.CandidateBinding.GID {
			return errors.Join(ErrBinding, err, requestErr)
		}
		if _, err := journal.Bytes(record); err != nil {
			return err
		}
	}
	body, err := r.reader.read(ctx, filepath.Join(r.journal, "generation-directory.json"), 4<<10, 0600, 0)
	var birth generationBirth
	if err != nil || decodeCanonical(body, 4<<10, &birth) != nil || birth.Schema != "ardents-endpoint-generation-directory-v1" || birth.GenerationDigest != r.intent.Candidate.GenerationDigest {
		return errors.Join(ErrBinding, err)
	}
	info := r.reader.directories[r.generation].Sys().(*syscall.Stat_t)
	if uint64(info.Dev) != birth.Device || info.Ino != birth.Inode {
		return ErrBinding
	}
	if body, err := r.reader.read(ctx, filepath.Join(r.journal, "completed-intent.json"), 128<<10, 0600, 0); !errors.Is(err, os.ErrNotExist) {
		wanted, encodeErr := canonicalJSON(r.intent)
		if err != nil || encodeErr != nil || !bytes.Equal(body, wanted) {
			return errors.Join(ErrBinding, err, encodeErr)
		}
		_, initiallyStopped := r.reader.files[filepath.Join(r.journal, "0007.json")]
		_, recoveredStopped := r.reader.files[filepath.Join(r.journal, "initial-recovered.json")]
		if !initiallyStopped && !recoveredStopped {
			return ErrBinding
		}
	}
	return nil
}

func (r *recoveryNative) readPrefix(ctx context.Context, filename string, mode os.FileMode, gid uint32, birth *fixedCreationRecord) error {
	info, err := os.Lstat(filename)
	if err != nil || info == nil || !info.Mode().IsRegular() {
		return errors.Join(ErrBinding, err)
	}
	native, ok := info.Sys().(*syscall.Stat_t)
	if !ok || native.Uid != 0 || native.Nlink != 1 || (info.Mode() != 0600 && info.Mode() != mode) || (native.Gid != 0 && native.Gid != gid) {
		return ErrBinding
	}
	if birth != nil && (uint64(native.Dev) != birth.Device || native.Ino != birth.Inode) {
		return ErrBinding
	}
	_, err = r.reader.readObserved(ctx, filename, 64<<20, info.Mode(), native.Gid, true)
	return err
}

func closeInitialRecovery(r *recoveryNative) error {
	if r == nil || r.reader == nil {
		return nil
	}
	return r.reader.lease.close()
}

func (r *recoveryNative) readDirectories(ctx context.Context) (returnedErr error) {
	worker := "/usr/lib/ardents/text-worker-root"
	wanted := map[string]os.FileMode{worker: 0700, "/run/ardents-text": 0710}
	workerEntries := map[string]bool{"ardents-text": true}
	for _, name := range []string{"dev", "proc", "sys", "run", "tmp", "etc", "root", "usr", "var", "var/tmp"} {
		wanted[filepath.Join(worker, name)] = 0555
		workerEntries[strings.Split(name, "/")[0]] = true
		r.inventories[filepath.Join(worker, name)] = map[string]bool{}
	}
	r.inventories[worker] = workerEntries
	r.inventories[filepath.Join(worker, "var")] = map[string]bool{"tmp": true}
	allowedParents := make(map[string]bool)
	for filename := range fixedResourceNames() {
		for parent := filepath.Dir(filename); parent != "/"; parent = filepath.Dir(parent) {
			allowedParents[parent] = true
		}
	}
	allowedParents["/etc/ardents"] = true
	directory := filepath.Join(r.journal, "directory-creations")
	info, err := os.Lstat(directory)
	if err != nil || !privateJournalDirectory(info) {
		return errors.Join(ErrBinding, err)
	}
	root, err := os.OpenRoot(directory)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, root.Close()) }()
	file, err := root.Open(".")
	if err != nil {
		return err
	}
	names, readErr := file.Readdirnames(65)
	closeErr := file.Close()
	if len(names) > 64 || (readErr != nil && !errors.Is(readErr, io.EOF)) || closeErr != nil {
		return errors.Join(ErrBinding, readErr, closeErr)
	}
	allowedNames := make(map[string]bool, len(names))
	for _, name := range names {
		allowedNames[name] = true
	}
	r.inventories[directory] = allowedNames
	births := make(map[string]bool)
	promoted := false
	for _, name := range names {
		body, err := r.reader.read(ctx, filepath.Join(directory, name), 4<<10, 0600, 0)
		var record fixedDirectoryCreation
		if err != nil || decodeCanonical(body, 4<<10, &record) != nil || record.Schema != "ardents-endpoint-directory-creation-v1" || record.GenerationDigest != r.intent.Candidate.GenerationDigest ||
			!canonicalPath(record.Path) || record.Device == 0 || record.Inode == 0 || record.PreviousMode != 0700 || record.PreviousGID != 0 {
			return errors.Join(ErrBinding, err)
		}
		promotion := name == digestHex([]byte(record.Path))+"-access.json"
		mode, required := wanted[record.Path]
		if promotion {
			if record.Path != worker || promoted || record.Mode != 0555 || record.GID != 0 {
				return ErrBinding
			}
			promoted = true
		} else {
			if name != digestHex([]byte(record.Path))+".json" || births[record.Path] || (!required && !allowedParents[record.Path]) {
				return ErrBinding
			}
			if !required {
				mode = 0755
			}
			gid := uint32(0)
			if record.Path == "/run/ardents-text" {
				gid = r.reader.gid
			}
			if record.Mode != uint32(mode) || record.GID != gid {
				return ErrBinding
			}
			births[record.Path] = true
		}
		if previous, exists := r.access[record.Path]; !exists || promotion {
			if exists && (previous.Device != record.Device || previous.Inode != record.Inode) {
				return ErrBinding
			}
			r.access[record.Path] = record
		} else if previous.Device != record.Device || previous.Inode != record.Inode {
			return ErrBinding
		}
	}
	if !promoted {
		return ErrBinding
	}
	for directory := range wanted {
		if !births[directory] {
			return ErrBinding
		}
	}
	for directory, record := range r.access {
		info, err := os.Lstat(directory)
		if err != nil || info == nil || !info.IsDir() {
			return errors.Join(ErrBinding, err)
		}
		native, ok := info.Sys().(*syscall.Stat_t)
		if !ok || native.Uid != 0 || uint64(native.Dev) != record.Device || native.Ino != record.Inode ||
			(info.Mode() != os.ModeDir|0700 && info.Mode() != os.ModeDir|os.FileMode(record.Mode)) || (native.Gid != 0 && native.Gid != record.GID) {
			return ErrBinding
		}
		r.reader.directories[directory] = info
	}
	return nil
}

func (r *recoveryNative) fixedDigests() (map[string]string, error) {
	result, artifact := make(map[string]string), make(map[string]string)
	for filename, name := range fixedResourceNames() {
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		digest := r.intent.CandidateBinding.Files[name]
		if !canonicalDigest(digest) {
			return nil, ErrBinding
		}
		result[filename] = digest
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			artifact[filename] = digest
		}
	}
	body, err := canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", artifact})
	if err != nil {
		return nil, err
	}
	result["/etc/ardents/text-worker-artifact.json"] = digestHex(body)
	body, err = canonicalJSON(r.intent.Candidate)
	if err != nil {
		return nil, err
	}
	result[filepath.Join(r.reader.lease.path, "selection.json")] = digestHex(body)
	return result, nil
}

func (r *recoveryNative) readFixedCreations(ctx context.Context) error {
	digests, err := r.fixedDigests()
	if err != nil {
		return err
	}
	wanted := make(map[string]bool, len(digests))
	for filename := range digests {
		wanted[digestHex([]byte(filename))+".json"] = true
	}
	directory := filepath.Join(r.journal, "creations")
	if err := recoveryInventory(directory, wanted); err != nil {
		return err
	}
	r.inventories[directory] = wanted
	for filename, digest := range digests {
		body, err := r.reader.read(ctx, filepath.Join(directory, digestHex([]byte(filename))+".json"), 4<<10, 0600, 0)
		var record fixedCreationRecord
		mode, gid := os.FileMode(0644), uint32(0)
		if filename == "/usr/lib/ardents/text-worker-root/ardents-text" {
			mode = 0555
		} else if filename == filepath.Join(r.reader.lease.path, "selection.json") {
			mode, gid = 0640, r.reader.gid
		}
		if err != nil || decodeCanonical(body, 4<<10, &record) != nil || record.Schema != "ardents-endpoint-installation-creation-v1" || record.GenerationDigest != r.intent.Candidate.GenerationDigest ||
			record.Path != filename || record.PreviousDigest != "" || record.CandidateDigest != digest || record.Mode != uint32(mode) || record.GID != gid || record.Device == 0 || record.Inode == 0 {
			return errors.Join(ErrBinding, err)
		}
		if err := r.readPrefix(ctx, filename, mode, gid, &record); err != nil {
			return err
		}
		r.creations[filename] = record
	}
	return nil
}

func (r *recoveryNative) observe(ctx context.Context) error {
	for directory, allowed := range r.inventories {
		if err := recoveryInventory(directory, allowed); err != nil {
			return err
		}
	}
	if err := r.reader.observe(ctx); err != nil {
		return err
	}
	if err := r.reader.observeAccountAndRoots(inspectedGeneration{binding: r.intent.CandidateBinding, request: r.intent.Request}); err != nil {
		return err
	}
	if err := observeAbsentInstallationUnits(ctx); err != nil {
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
		if err := verifyStoppedEndpointProperties(version, unit, service, r.intent.Request, r.intent.Candidate.GenerationDigest); err != nil {
			return err
		}
	}
	return cgroup.ObserveEmpty(ctx)
}

func completeInitialRecovery(ctx context.Context, owner *recoveryOperation, authorization Authorization) (result ProvisionResult, returnedErr error) {
	r := owner.native
	defer func() {
		if returnedErr != nil {
			bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			filename := filepath.Join(r.journal, "recovery-failure.json")
			var err error
			if _, retained := r.reader.files[filename]; retained {
				err = r.reader.syncObserved(bounded, filename)
			} else if archive := filepath.Join(r.journal, "recovery-failure-archived.json"); r.reader.files[archive].identity != nil {
				// All retries still belong to this immutable intent. Keep its
				// first failure rather than manufacturing a conflicting second
				// archive. The current refusal remains in the returned outcome.
				err = r.reader.syncObserved(bounded, archive)
				if err == nil {
					err = r.reader.writePrivate(bounded, filename, r.reader.files[archive].body)
				}
			} else {
				var body []byte
				body, err = canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: r.intent.Candidate.GenerationDigest, BindingDigest: r.intent.Candidate.BindingDigest, Phase: "initial-recovery-failed", OriginalError: returnedErr.Error()})
				if err == nil {
					err = r.reader.writePrivate(bounded, filename, body)
				}
			}
			returnedErr = errors.Join(ErrRepairRequired, returnedErr, err)
		}
	}()
	checked, err := recoveryGeneration(ctx, r.intent, owner.reference, authorization)
	if err != nil {
		return ProvisionResult{}, err
	}
	if _, err := readGenerationFileBirths(ctx, r.reader, r.journal, r.intent.Candidate, checked.binding, checked.files, r.inventories[r.journal]); err != nil {
		return ProvisionResult{}, err
	}
	wanted := make(map[string][]byte, 25)
	for name, body := range checked.files {
		wanted[filepath.Join(r.generation, name)] = body
	}
	bindingBody, err := canonicalJSON(checked.binding)
	if err != nil {
		return ProvisionResult{}, err
	}
	wanted[filepath.Join(r.generation, "binding.json")] = bindingBody
	artifact := make(map[string]string)
	for filename, name := range fixedResourceNames() {
		if name == "ardents-endpoint.service" {
			name = "endpoint-unit.service"
		}
		wanted[filename] = checked.files[name]
		if name != "endpoint-unit.service" && name != "ardents-text.conf" {
			artifact[filename] = digestHex(checked.files[name])
		}
	}
	wanted["/etc/ardents/text-worker-artifact.json"], err = canonicalJSON(struct {
		Schema string            `json:"schema"`
		Files  map[string]string `json:"files"`
	}{"ardents-text-worker-artifact-v1", artifact})
	if err != nil {
		return ProvisionResult{}, err
	}
	wanted[filepath.Join(r.reader.lease.path, "selection.json")], err = canonicalJSON(checked.selected)
	if err != nil {
		return ProvisionResult{}, err
	}
	// Validate every prefix before the first repair. A same-sized foreign file,
	// mismatching proof or incomplete resource set cannot cause partial effects.
	for filename, body := range wanted {
		observed, exists := r.reader.files[filename]
		if !exists || len(body) == 0 || len(observed.body) > len(body) || !bytes.Equal(observed.body, body[:len(observed.body)]) {
			return ProvisionResult{}, ErrBinding
		}
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, err
	}
	// Re-establish durability of the original intent and both birth inventories
	// before changing any recorded object. Visibility is not a prior sync receipt.
	metadata := make([]string, 0, len(r.reader.files))
	for filename := range r.reader.files {
		if strings.HasPrefix(filename, r.journal+string(filepath.Separator)) || filename == filepath.Join(r.reader.lease.path, "transition.json") || strings.HasPrefix(filename, filepath.Join(r.reader.lease.path, "preparation")+string(filepath.Separator)) {
			metadata = append(metadata, filename)
		}
	}
	sort.Strings(metadata)
	for _, filename := range metadata {
		if err := r.reader.syncObserved(ctx, filename); err != nil {
			return ProvisionResult{}, err
		}
	}
	directories := make([]string, 0, len(r.access))
	for filename := range r.access {
		directories = append(directories, filename)
	}
	sort.Strings(directories)
	for _, filename := range directories {
		if err := r.repairDirectory(ctx, filename); err != nil {
			return ProvisionResult{}, err
		}
	}
	filenames := make([]string, 0, len(wanted))
	for filename := range wanted {
		filenames = append(filenames, filename)
	}
	sort.Strings(filenames)
	for _, filename := range filenames {
		mode, gid := os.FileMode(0640), r.reader.gid
		if record, fixed := r.creations[filename]; fixed {
			mode, gid = os.FileMode(record.Mode), record.GID
		} else if filepath.Base(filename) == "ardents-linux-amd64" || filepath.Base(filename) == "ardents-text-linux-amd64" {
			mode = 0555
		}
		if err := r.repairFile(ctx, filename, wanted[filename], mode, gid); err != nil {
			return ProvisionResult{}, err
		}
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, err
	}
	if err := systemd.Reload(ctx); err != nil {
		return ProvisionResult{}, err
	}
	if err := observeStoppedManager(ctx); err != nil {
		return ProvisionResult{}, err
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, err
	}
	body, err := canonicalJSON(generationTransition{Schema: "ardents-endpoint-installation-transition-v1", GenerationDigest: checked.selected.GenerationDigest, BindingDigest: checked.selected.BindingDigest, Phase: "initial-recovered-stopped"})
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := r.reader.writePrivate(ctx, filepath.Join(r.journal, "initial-recovered.json"), body); err != nil {
		return ProvisionResult{}, err
	}
	body, err = canonicalJSON(r.intent)
	if err != nil {
		return ProvisionResult{}, err
	}
	if err := r.reader.writePrivate(ctx, filepath.Join(r.journal, "completed-intent.json"), body); err != nil {
		return ProvisionResult{}, err
	}
	for source, target := range map[string]string{
		filepath.Join(r.reader.lease.path, "preparation", "failure.json"): "original-preparation-failure-archived.json",
		filepath.Join(r.journal, "original-transition-failure.json"):      "original-transition-failure-archived.json",
		filepath.Join(r.journal, "recovery-failure.json"):                 "recovery-failure-archived.json",
	} {
		if original, exists := r.reader.files[source]; exists {
			if err := r.reader.writePrivate(ctx, filepath.Join(r.journal, target), original.body); err != nil {
				return ProvisionResult{}, err
			}
			if err := r.reader.removeObserved(ctx, source); err != nil {
				return ProvisionResult{}, err
			}
		}
	}
	if _, retained := r.reader.files[filepath.Join(r.reader.lease.path, "transition.json")]; retained {
		if err := r.reader.removeObserved(ctx, filepath.Join(r.reader.lease.path, "transition.json")); err != nil {
			return ProvisionResult{}, err
		}
	}
	if err := r.observe(ctx); err != nil {
		return ProvisionResult{}, err
	}
	return ProvisionResult{Status: "installed-recovered-stopped", GenerationDigest: checked.selected.GenerationDigest}, ctx.Err()
}

func (r *recoveryNative) repairDirectory(ctx context.Context, filename string) (returnedErr error) {
	if err := r.observe(ctx); err != nil {
		return err
	}
	record := r.access[filename]
	before := r.reader.directories[filename]
	if before.Mode() == os.ModeDir|os.FileMode(record.Mode) && before.Sys().(*syscall.Stat_t).Gid == record.GID {
		return nil
	}
	file, err := os.OpenFile(filename, os.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close(), ctx.Err()) }()
	info, err := file.Stat()
	if err != nil || !sameObservedDirectory(before, info) {
		return errors.Join(ErrBinding, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := file.Chown(0, int(record.GID)); err != nil {
		return err
	}
	if err := file.Chmod(os.FileMode(record.Mode)); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := file.Stat()
	pathInfo, pathErr := os.Lstat(filename)
	if err != nil || pathErr != nil || !os.SameFile(before, after) || !sameObservedDirectory(after, pathInfo) || after.Mode() != os.ModeDir|os.FileMode(record.Mode) || after.Sys().(*syscall.Stat_t).Gid != record.GID {
		return errors.Join(ErrBinding, err, pathErr)
	}
	r.reader.directories[filename] = after
	return syncDirectDirectory(filepath.Dir(filename))
}

func (r *recoveryNative) repairFile(ctx context.Context, filename string, body []byte, mode os.FileMode, gid uint32) (returnedErr error) {
	if err := r.observe(ctx); err != nil {
		return err
	}
	observed := r.reader.files[filename]
	if bytes.Equal(observed.body, body) && observed.mode == mode && observed.gid == gid {
		return r.reader.syncObserved(ctx, filename)
	}
	file, err := os.OpenFile(filename, os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if err != nil {
		return err
	}
	defer func() { returnedErr = errors.Join(returnedErr, file.Close(), ctx.Err()) }()
	info, err := file.Stat()
	if err != nil || !sameObservedFile(observed.identity, info) {
		return errors.Join(ErrBinding, err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	written, err := file.Write(body)
	if err != nil || written != len(body) {
		return errors.Join(io.ErrShortWrite, err)
	}
	if err := file.Chown(0, int(gid)); err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return err
	}
	if err := file.Sync(); err != nil {
		return err
	}
	after, err := file.Stat()
	pathInfo, pathErr := os.Lstat(filename)
	if err != nil || pathErr != nil || !os.SameFile(observed.identity, after) || !sameObservedFile(after, pathInfo) || after.Mode() != mode || after.Sys().(*syscall.Stat_t).Gid != gid || after.Size() != int64(len(body)) {
		return errors.Join(ErrBinding, err, pathErr)
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	actual, err := io.ReadAll(io.LimitReader(file, int64(len(body))+1))
	if err != nil || !bytes.Equal(actual, body) {
		return errors.Join(ErrBinding, err)
	}
	r.reader.files[filename] = fileObservation{identity: after, body: bytes.Clone(body), mode: mode, gid: gid}
	return syncDirectDirectory(filepath.Dir(filename))
}
