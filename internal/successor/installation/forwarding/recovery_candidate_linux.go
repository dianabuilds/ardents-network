package forwarding

import (
	"bytes"
	"context"
	"path/filepath"

	"github.com/dianabuilds/ardents-network/internal/successor/installation/infrastructure"
)

// Exact candidate reconstruction is separate from safe-prefix/native admission.
// In particular, it grants no permission to replay a birth, reload or Start.
type recoveryCandidate struct {
	selection generationSelection
	files     map[string][]byte
	unit      []byte
	hashes    map[string]string
}

func prepareRecoveryCandidate(a infrastructure.ForwardingAuthorization, input Inputs, selected generationSelection, original *recoveryOriginal) (*recoveryCandidate, error) {
	p := input.original
	if p == nil || len(p.files) != 4 || len(selected.raw) == 0 || original == nil || original.prefix == nil || !bytes.Equal(original.cut.Boundary.Binding, selected.raw) {
		return nil, ErrInput
	}
	if _, initial := a.InitialFacts(); initial {
		return nil, infrastructure.ErrAuthorization
	}
	b := selected.binding
	incompleteArtifacts := original.cut.Boundary.Operation == "initial" && original.cut.Boundary.Records >= 72 && original.cut.Boundary.Records <= 107
	if incompleteArtifacts && (selected.replacement != nil || original.cut.Boundary.Journal != b.Request.InstallationRoot+".forwarding-initial-journal" || original.cut.Origin.ProgramSHA256 != b.ProgramSHA256) {
		return nil, ErrBinding
	}
	if p.request != b.Request || digest(a.Descriptor()) != b.Generation || digest(a.Program()) != b.ProgramSHA256 || digest(a.Template()) != b.TemplateSHA256 || digest(p.files[0].raw) != b.RequestSHA256 || digest(p.files[1].raw) != b.PlanSHA256 || digest(p.files[2].raw) != b.CertificateSHA256 {
		return nil, ErrBinding
	}
	unit, err := RenderUnit(a.Template(), p.request, b.Generation)
	if err != nil || digest(unit) != b.UnitSHA256 {
		return nil, ErrBinding
	}
	files := a.Artifacts()
	if len(files) != 8 {
		return nil, ErrBinding
	}
	// Fresh permission may advance metadata floors for the same payloads. Keep
	// the original sealed metadata as history rather than replacing its bytes.
	sealed := filepath.Join(b.Request.InstallationRoot, "generations", b.Generation)
	var metadataErr error
	if incompleteArtifacts {
		// Choose from the original typed boundary before reading files. The
		// unfinished metadata is only a proposal: the complete original inventory
		// below must commit exactly those bytes. A read error never selects it.
		metadataErr = restoreRecoveryInitialMetadata(p.ctx, sealed, files, original.cut.Boundary.Records)
	} else {
		metadataErr = restoreRecoveryMetadata(p.ctx, sealed, files)
	}
	if metadataErr != nil {
		for _, body := range files {
			clear(body)
		}
		return nil, metadataErr
	}
	files["forwarding-request.json"], files["forwarding-plan.json"] = bytes.Clone(p.files[0].raw), bytes.Clone(p.files[1].raw)
	files["rendered-forwarding.service"], files["binding.json"] = unit, bytes.Clone(selected.raw)
	if _, err := validateSelectedGeneration(b, selected.raw, files); err != nil {
		for _, raw := range files {
			clear(raw)
		}
		return nil, err
	}
	hashes := map[string]string{}
	for name, raw := range files {
		hashes[filepath.Join(sealed, name)] = digest(raw)
	}
	hashes[filepath.Join(b.Request.InstallationRoot, "binding.json")] = digest(selected.raw)
	hashes[filepath.Join(b.Request.CredentialDirectory, "certificate.pem")] = digest(p.files[2].raw)
	hashes[filepath.Join(b.Request.CredentialDirectory, "private-key.pem")] = digest(p.files[3].raw)
	hashes["/etc/systemd/system/ardents-forwarding.service"] = digest(unit)
	if selected.replacement == nil {
		pending := []byte("ardents-forwarding-initial-pending-v1\n" + digest(selected.raw) + "\n")
		hashes[b.Request.InstallationRoot+".forwarding-initial-pending"] = digest(pending)
	} else {
		hashes[replacementPending(b.Request.InstallationRoot)] = digest(replacementPendingBytes(selected.raw))
	}
	return &recoveryCandidate{selected, files, unit, hashes}, nil
}

// Only historical byte/native reads. The actual caller supplies fresh opaque
// artifact authority and independently verifies the complete original binding.
func restoreRecoveryMetadata(ctx context.Context, sealed string, files map[string][]byte) error {
	return restoreRecoveryMetadataFiles(ctx, sealed, files, []string{"root.json", "timestamp.json", "snapshot.json", "targets.json", "infrastructure-enrollment.json"})
}

// The caller selects an original generation-file boundary before any reads.
// Only original file-ready metadata is historical input. Missing, intended or
// empty-born leaves keep their detached proposals; read errors never select a
// proposal. Enrollment's complete canonical inventory and the original binding
// must still commit every proposed byte. This mechanism grants no birth right.
func restoreRecoveryInitialMetadata(ctx context.Context, sealed string, files map[string][]byte, records uint32) error {
	if records < 72 || records > 107 {
		return ErrBinding
	}
	names := []string{}
	for _, metadata := range []struct {
		name  string
		ready uint32
	}{
		{"infrastructure-enrollment.json", 90},
		{"root.json", 99},
		{"snapshot.json", 102},
		{"targets.json", 105},
		{"timestamp.json", 108},
	} {
		if records >= metadata.ready {
			names = append(names, metadata.name)
		}
	}
	return restoreRecoveryMetadataFiles(ctx, sealed, files, names)
}

func restoreRecoveryMetadataFiles(ctx context.Context, sealed string, files map[string][]byte, names []string) error {
	if ctx == nil || files == nil || !validPath(sealed) {
		return ErrInput
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, name := range names {
		limit := int64(1 << 20)
		if name == "infrastructure-enrollment.json" {
			limit = 16 << 10
		}
		raw, _, err := readInitialOriginal(ctx, filepath.Join(sealed, name), limit, 0444, 0, 0, false)
		if err != nil {
			return err
		}
		clear(files[name])
		files[name] = raw
	}
	return nil
}

func (c *recoveryCandidate) clear() {
	if c == nil {
		return
	}
	for _, raw := range c.files {
		clear(raw)
	}
	clear(c.unit)
}
