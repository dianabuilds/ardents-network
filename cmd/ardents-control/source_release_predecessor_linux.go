package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/successor/enrollment"
	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
	"github.com/dianabuilds/ardents-network/internal/successor/release"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// This finite public predecessor is held by preparation's original file
// custody. It is neither the installation's trust history nor an authorization.
// Release authenticates both generations through its own diagnostic verifier.
type sourceReleasePredecessor struct {
	files                                             map[string][]byte
	root                                              *metadata.Metadata[metadata.RootType]
	timestampVersion, snapshotVersion, targetsVersion int64
	check                                             func() error
	candidate                                         enrollment.InfrastructureCandidate
}

func readSourceReleasePredecessor(inputs *sourceControlInputs, path string, plan sourceReleasePlan, candidate []byte) (*sourceReleasePredecessor, error) {
	directory, err := inputs.directory(path)
	if err != nil {
		return nil, err
	}
	names := []string{infrastructure.ProgramName, infrastructure.TemplateName, infrastructure.DescriptorName, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"}
	check := func() error {
		if _, err := directory.file.Seek(0, io.SeekStart); err != nil {
			return err
		}
		found, err := directory.file.Readdirnames(len(names) + 1)
		if err != nil && err != io.EOF || len(found) != len(names) {
			return errSourceControlInput
		}
		for _, name := range found {
			if !slices.Contains(names, name) {
				return errSourceControlInput
			}
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, err
	}
	p := &sourceReleasePredecessor{files: map[string][]byte{}, check: check}
	p.candidate, err = enrollment.ReadInfrastructureCandidate(inputs.ctx, path)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		limit := int64(1 << 20)
		if name == infrastructure.ProgramName {
			limit = 64 << 20
		}
		p.files[name], err = inputs.read(filepath.Join(path, name), limit)
		if err != nil {
			return nil, err
		}
	}
	previous, err := infrastructure.ValidateResources(p.files[infrastructure.DescriptorName], p.files[infrastructure.ProgramName], p.files[infrastructure.TemplateName])
	if err != nil {
		return nil, err
	}
	if plan.Schema == "ardents-source-timestamp-release-plan-v1" {
		if !bytes.Equal(candidate, p.files[infrastructure.DescriptorName]) {
			return nil, errSourceControlInput
		}
	} else {
		next, err := infrastructure.DecodeDescriptor(candidate)
		if plan.Schema != "ardents-source-forward-release-plan-v1" || err != nil || next.ReleaseVersion <= previous.ReleaseVersion {
			return nil, errors.Join(errSourceControlInput, err)
		}
	}
	p.root, err = metadata.Root().FromBytes(p.files["root.json"])
	if err != nil {
		return nil, err
	}
	for name, destination := range map[string]*int64{"timestamp.json": &p.timestampVersion, "snapshot.json": &p.snapshotVersion, "targets.json": &p.targetsVersion} {
		var version struct {
			Signed struct {
				Version int64 `json:"version"`
			} `json:"signed"`
		}
		// The ordinary Release verifier below authenticates the entire metadata,
		// including duplicate-field and closed-role constraints. This projection
		// only bounds arithmetic before constructing the new fixed-purpose set.
		if err := json.Unmarshal(p.files[name], &version); err != nil {
			return nil, err
		}
		if version.Signed.Version < 1 || version.Signed.Version == math.MaxInt64 && (plan.Schema != "ardents-source-timestamp-release-plan-v1" || name == "timestamp.json") {
			return nil, errSourceControlInput
		}
		*destination = version.Signed.Version
	}
	return p, errors.Join(check(), inputs.checkProtected(), p.candidate.Check())
}

// Exact retained resources and custom declarations, rather than an equal version
// or detached identifier, delimit timestamp refresh. The original diagnostic
// verifier still authenticates both targets before any signature is produced.
func (p *sourceReleasePredecessor) checkTimestampResources(plan sourceReleasePlan, artifacts [3][]byte) error {
	for i, name := range []string{infrastructure.ProgramName, infrastructure.DescriptorName, infrastructure.TemplateName} {
		if !bytes.Equal(artifacts[i], p.files[name]) {
			return errSourceControlInput
		}
	}
	targets, err := metadata.Targets().FromBytes(p.files["targets.json"])
	if err != nil || len(targets.Signed.Targets) != 2 {
		return errors.Join(errSourceControlInput, err)
	}
	snapshot, err := metadata.Snapshot().FromBytes(p.files["snapshot.json"])
	if err != nil {
		return err
	}
	if plan.Expires.After(p.root.Signed.Expires) || plan.Expires.After(targets.Signed.Expires) || plan.Expires.After(snapshot.Signed.Expires) {
		return errSourceControlInput
	}
	for i, name := range []string{infrastructure.ProgramTarget, infrastructure.GenerationTarget} {
		target := targets.Signed.Targets[name]
		if target == nil || target.Custom == nil {
			return errSourceControlInput
		}
		requested := []json.RawMessage{plan.ProgramCustom, plan.GenerationCustom}[i]
		var original, proposed bytes.Buffer
		if json.Compact(&original, *target.Custom) != nil || json.Compact(&proposed, requested) != nil || !bytes.Equal(original.Bytes(), proposed.Bytes()) {
			return errSourceControlInput
		}
	}
	return nil
}

func (p *sourceReleasePredecessor) matchKeys(ids []string) error {
	if len(p.root.Signed.Keys) != 5 || len(p.root.Signed.Roles) != len(metadata.TOP_LEVEL_ROLE_NAMES) {
		return errSourceControlInput
	}
	for _, id := range ids {
		if p.root.Signed.Keys[id] == nil {
			return errSourceControlInput
		}
	}
	for _, name := range metadata.TOP_LEVEL_ROLE_NAMES {
		role := p.root.Signed.Roles[name]
		if role == nil || role.Threshold != 3 || len(role.KeyIDs) != len(ids) {
			return errSourceControlInput
		}
		for _, id := range ids {
			if !slices.Contains(role.KeyIDs, id) {
				return errSourceControlInput
			}
		}
	}
	return nil
}

func (p *sourceReleasePredecessor) authenticate(ctx context.Context, v *release.Verifier, plan sourceReleasePlan) error {
	files, err := release.StaticMetadataFiles(p.files["root.json"], p.files["timestamp.json"], p.files["snapshot.json"], p.files["targets.json"])
	if err != nil {
		return err
	}
	var proofs []release.Authorization
	for i, target := range []string{infrastructure.ProgramTarget, infrastructure.GenerationTarget} {
		artifact := p.files[infrastructure.ProgramName]
		if i == 1 {
			artifact = p.files[infrastructure.DescriptorName]
		}
		decision := v.Evaluate(ctx, release.Inputs{RootBytes: p.files["root.json"], Files: files, TargetPath: target, Artifact: artifact,
			Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: plan.Network, RefTime: plan.Reference}})
		proof, ok := decision.Authorization()
		if !ok {
			return errors.Join(errSourceControlInput, decision.Err())
		}
		proofs = append(proofs, proof)
	}
	if err := v.CheckAuthorizations(ctx, proofs...); err != nil {
		return err
	}
	// Reevaluate through Installation's actual coherent-pair consumer. This
	// binds both signed target identities to the exact predecessor descriptor
	// after the seed evaluations have established complete diagnostic floors.
	a, err := infrastructure.AuthenticateCandidate(ctx, v, p.candidate, release.Inputs{
		RootBytes: p.files["root.json"], Files: files, TargetPath: infrastructure.ProgramTarget, Artifact: p.files[infrastructure.ProgramName],
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: plan.Network, RefTime: plan.Reference}})
	if err != nil {
		return err
	}
	return a.Check(ctx, v)
}
