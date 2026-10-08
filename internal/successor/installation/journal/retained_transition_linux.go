package journal

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
)

// RetainedRecord is a detached physical observation, never journal admission.
// Installation verifies its schema, phase and provenance under its own writer.
type RetainedRecord struct {
	Identity os.FileInfo
	Bytes    []byte
}

// RetainedReplacements describes one independently observed original group.
// Installation admits its closed record prefix and unchanged fixed preimages.
// Empty groups have real directory custody too; presence grants no mutation.
type RetainedReplacements struct {
	Identity os.FileInfo
	Records  map[string]RetainedRecord
}

// OpenTransition retains an independently observed, closed journal prefix.
// An optional exact replacement group is observed separately; other groups and
// unobserved records refuse. It creates nothing. A real
// Installation recovery decides whether this prefix may later be continued.
func OpenTransition(ctx context.Context, parentPath string, parentIdentity, directoryIdentity os.FileInfo, name string, records map[string]RetainedRecord, replacements ...RetainedReplacements) (result *Transition, returnedErr error) {
	if ctx == nil || os.Geteuid() != 0 || !canonicalPath(parentPath) || filepath.Base(parentPath) != "journals" || !canonicalDigest(name) || !privateJournalDirectory(parentIdentity) || !privateJournalDirectory(directoryIdentity) || len(records) == 0 || len(records) > 31 {
		return nil, ErrInput
	}
	ordinary, births := 0, 0
	for name, record := range records {
		if generationFileRecord(name) {
			births++
			if len(record.Bytes) > 4<<10 {
				return nil, ErrInput
			}
		} else {
			ordinary++
		}
	}
	// Preserve the original sixteen-record transition limit separately from
	// the fifteen artifact-birth slots; neither namespace lends spare capacity.
	if ordinary > 16 || births > 15 {
		return nil, ErrInput
	}
	if len(replacements) > 1 || len(replacements) == 1 && (!privateJournalDirectory(replacements[0].Identity) || len(replacements[0].Records) > 16) {
		return nil, ErrInput
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	parent, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, err
	}
	j := &Transition{parent: parent, parentPath: parentPath, parentIdentity: parentIdentity, name: name, groups: make(map[Collection]*transitionDirectory)}
	defer func() {
		if returnedErr != nil {
			j.failure = returnedErr
			returnedErr = j.Close()
			result = nil
		}
	}()
	if err := j.observeParent(); err != nil {
		return nil, err
	}
	current, err := parent.Lstat(name)
	if err != nil || !transitionDirectoryMatches(directoryIdentity, current) {
		return nil, errors.Join(ErrBinding, err)
	}
	dir := &transitionDirectory{parent: parent, name: name, records: make(map[string]recordObservation), children: make(map[string]*transitionDirectory)}
	j.directory = dir
	for name, expected := range records {
		if !transitionName(Transitions, name) || len(expected.Bytes) == 0 || len(expected.Bytes) > 128<<10 || name != "completed-intent.json" && len(expected.Bytes) > 64<<10 {
			return nil, ErrInput
		}
		record := recordObservation{identity: expected.Identity, body: bytes.Clone(expected.Bytes)}
		if !transitionRecordMatches(record, expected.Identity) {
			return nil, ErrBinding
		}
		dir.records[name] = record
	}
	if err := dir.openHandles(); err != nil {
		return nil, err
	}
	if !transitionDirectoryMatches(directoryIdentity, dir.identity) {
		return nil, ErrBinding
	}
	if len(replacements) == 1 {
		expected := replacements[0]
		group := &transitionDirectory{parent: dir.root, name: "replacements", records: make(map[string]recordObservation), children: make(map[string]*transitionDirectory)}
		j.groups[Replacements] = group
		j.order = append(j.order, group)
		dir.children[group.name] = group
		for name, expected := range expected.Records {
			if !transitionName(Replacements, name) || len(expected.Bytes) == 0 || len(expected.Bytes) > 64<<10 {
				return nil, ErrInput
			}
			record := recordObservation{identity: expected.Identity, body: bytes.Clone(expected.Bytes)}
			if !transitionRecordMatches(record, expected.Identity) {
				return nil, ErrBinding
			}
			group.records[name] = record
		}
		if err := group.open(); err != nil {
			return nil, err
		}
		if !transitionDirectoryMatches(expected.Identity, group.identity) {
			return nil, ErrBinding
		}
	}
	return j, errors.Join(j.observePhysical(), ctx.Err())
}

// ResyncReplacements makes the retained original group and its directory links
// durable, including an empty group, before Installation's admitted effects.
// It does not establish creation ownership or synchronize unobserved records.
func (j *Transition) ResyncReplacements(ctx context.Context) (returnedErr error) {
	if ctx == nil || j == nil || j.groups[Replacements] == nil {
		return ErrInput
	}
	if j.failure != nil {
		return j.failure
	}
	defer func() {
		if returnedErr != nil {
			j.failure = returnedErr
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := j.observePhysical(); err != nil {
		return err
	}
	return errors.Join(j.groups[Replacements].file.Sync(), j.directory.file.Sync(), transitionSync(j.parent), j.observePhysical(), ctx.Err())
}

// RetireGenerationFailure removes only the original phase slot after an exact
// separately retained first-error copy is durable. Installation admits the
// failed-phase grammar and fresh proofs; this operation grants no continuation.
func (j *Transition) RetireGenerationFailure(ctx context.Context) (returnedErr error) {
	if j == nil || ctx == nil {
		return ErrInput
	}
	if j.failure != nil {
		return j.failure
	}
	defer func() {
		if returnedErr != nil {
			j.failure = returnedErr
		}
	}()
	if err := j.observePhysical(); err != nil {
		return err
	}
	phase, phasePresent := j.directory.records["0002.json"]
	first, firstPresent := j.directory.records["original-transition-failure.json"]
	if !phasePresent || !firstPresent || !bytes.Equal(phase.body, first.body) {
		return ErrBinding
	}
	for _, name := range []string{"0002.json", "original-transition-failure.json"} {
		if err := j.Resync(ctx, Transitions, name); err != nil {
			return err
		}
	}
	if err := errors.Join(j.observePhysical(), ctx.Err()); err != nil {
		return err
	}
	if err := j.directory.root.Remove("0002.json"); err != nil {
		return err
	}
	// Retain the actual deletion even if its directory sync fails. A fresh
	// opening must independently admit the retained copy and missing phase.
	delete(j.directory.records, "0002.json")
	return errors.Join(j.directory.file.Sync(), j.observePhysical(), ctx.Err())
}
