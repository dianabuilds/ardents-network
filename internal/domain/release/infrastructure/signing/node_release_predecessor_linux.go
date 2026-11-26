package signing

import (
	"context"
	"encoding/json"
	"errors"
	artifactio "github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"math"
	"path/filepath"
	"slices"

	"github.com/dianabuilds/ardents-network/internal/domain/enrollment"
	"github.com/dianabuilds/ardents-network/internal/domain/release"
	"github.com/dianabuilds/ardents-network/internal/domain/release/infrastructure"
	"github.com/theupdateframework/go-tuf/v2/metadata"
)

// Original public custody only. A predecessor's detached versions are used for
// bounded arithmetic, then genuine coherent Release verification must succeed
// before any forward signature. No Source signing purpose or installed trust
// lease is reused by these fixed-role producers.
type nodeReleasePredecessor struct {
	role                                              nodeReleaseRole
	files                                             map[string][]byte
	root                                              *metadata.Metadata[metadata.RootType]
	timestampVersion, snapshotVersion, targetsVersion int64
	check                                             func() error
	candidate                                         enrollment.InfrastructureCandidate
}

func readNodeReleasePredecessor(ctx context.Context, role nodeReleaseRole, inputs *artifactio.Inputs, path string, plan fixedNodeReleasePlan, candidateVersion int64) (*nodeReleasePredecessor, error) {
	namesForRole, ok := role.artifacts()
	if !ok || plan.Schema != role.schema(nodeReleaseForward) {
		return nil, artifactio.ErrInput
	}
	directory, err := inputs.Directory(path)
	if err != nil {
		return nil, err
	}
	names := []string{infrastructure.ProgramName, namesForRole.template, namesForRole.descriptor, "infrastructure-enrollment.json", "root.json", "timestamp.json", "snapshot.json", "targets.json"}
	check := func() error {
		if _, err := directory.Descriptor().Seek(0, io.SeekStart); err != nil {
			return err
		}
		found, err := directory.Descriptor().Readdirnames(len(names) + 1)
		if err != nil && err != io.EOF || len(found) != len(names) {
			return errors.Join(artifactio.ErrInput, err)
		}
		for _, name := range found {
			if !slices.Contains(names, name) {
				return artifactio.ErrInput
			}
		}
		return nil
	}
	if err := check(); err != nil {
		return nil, err
	}
	p := &nodeReleasePredecessor{role: role, files: map[string][]byte{}, check: check}
	p.candidate, err = enrollment.ReadInfrastructureCandidate(ctx, path)
	if err != nil {
		return nil, err
	}
	for _, name := range names {
		limit := int64(1 << 20)
		if name == infrastructure.ProgramName {
			limit = 64 << 20
		}
		p.files[name], err = inputs.Read(filepath.Join(path, name), limit)
		if err != nil {
			return nil, err
		}
	}
	previous, err := infrastructure.ValidateRuntimeResources(string(role), p.files[namesForRole.descriptor], p.files[infrastructure.ProgramName], p.files[namesForRole.template])
	if err != nil {
		return nil, err
	}
	if candidateVersion <= previous.ReleaseVersion {
		return nil, artifactio.ErrInput
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
		if err := json.Unmarshal(p.files[name], &version); err != nil {
			return nil, err
		}
		if version.Signed.Version < 1 || version.Signed.Version == math.MaxInt64 {
			return nil, artifactio.ErrInput
		}
		*destination = version.Signed.Version
	}
	return p, errors.Join(check(), inputs.CheckProtected(), p.candidate.Check())
}

func (p *nodeReleasePredecessor) matchKeys(ids []string) error {
	if len(p.root.Signed.Keys) != 5 || len(ids) != 5 || len(p.root.Signed.Roles) != len(metadata.TOP_LEVEL_ROLE_NAMES) {
		return artifactio.ErrInput
	}
	for _, id := range ids {
		if p.root.Signed.Keys[id] == nil {
			return artifactio.ErrInput
		}
	}
	for _, name := range metadata.TOP_LEVEL_ROLE_NAMES {
		role := p.root.Signed.Roles[name]
		if role == nil || role.Threshold != 3 || len(role.KeyIDs) != len(ids) {
			return artifactio.ErrInput
		}
		for _, id := range ids {
			if !slices.Contains(role.KeyIDs, id) {
				return artifactio.ErrInput
			}
		}
	}
	return nil
}

func (p *nodeReleasePredecessor) authenticate(ctx context.Context, v *release.Verifier, plan fixedNodeReleasePlan) error {
	namesForRole, ok := p.role.artifacts()
	if !ok || plan.Schema != p.role.schema(nodeReleaseForward) {
		return artifactio.ErrInput
	}
	files, err := release.StaticMetadataFiles(p.files["root.json"], p.files["timestamp.json"], p.files["snapshot.json"], p.files["targets.json"])
	if err != nil {
		return err
	}
	var proofs []release.Authorization
	for n, target := range []string{infrastructure.ProgramTarget, namesForRole.target} {
		artifact := p.files[infrastructure.ProgramName]
		if n == 1 {
			artifact = p.files[namesForRole.descriptor]
		}
		decision := v.Evaluate(ctx, release.Inputs{RootBytes: p.files["root.json"], Files: files, TargetPath: target, Artifact: artifact,
			Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: plan.Network, RefTime: plan.Reference}})
		proof, ok := decision.Authorization()
		if !ok {
			return errors.Join(artifactio.ErrInput, decision.Err())
		}
		proofs = append(proofs, proof)
	}
	if err := v.CheckAuthorizations(ctx, proofs...); err != nil {
		return err
	}
	input := release.Inputs{RootBytes: p.files["root.json"], Files: files,
		TargetPath: infrastructure.ProgramTarget, Artifact: p.files[infrastructure.ProgramName],
		Local: release.LocalEnvironment{Platform: "linux-amd64", Architecture: "amd64", Environment: "alpha", Network: plan.Network, RefTime: plan.Reference}}
	var checkAuthorization func(context.Context, *release.Verifier) error
	switch p.role {
	case forwardingReleaseRole:
		authorization, err := infrastructure.AuthenticateForwardingCandidate(ctx, v, p.candidate, input)
		if err != nil {
			return err
		}
		checkAuthorization = authorization.Check
	case joinReleaseRole:
		authorization, err := infrastructure.AuthenticateJoinCandidate(ctx, v, p.candidate, input)
		if err != nil {
			return err
		}
		checkAuthorization = authorization.Check
	case descriptorReleaseRole:
		authorization, err := infrastructure.AuthenticateDescriptorCandidate(ctx, v, p.candidate, input)
		if err != nil {
			return err
		}
		checkAuthorization = authorization.Check
	default:
		return artifactio.ErrInput
	}
	return errors.Join(checkAuthorization(ctx, v), p.check(), p.candidate.Check())
}
