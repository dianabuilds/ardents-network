package release

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/theupdateframework/go-tuf/v2/metadata"
	"path/filepath"
	"testing"
)

func TestTwoFreshTargetsShareMetadataAndRejectSubstitutedGeneration(t *testing.T) {
	h := freshSignedHistory(t)
	targets, err := metadata.Targets().FromBytes(h.input.Files[metadataBaseURL+"1.targets.json"])
	if err != nil {
		t.Fatal(err)
	}
	template := targets.Signed.Targets[h.input.TargetPath]
	var fields map[string]any
	if err = json.Unmarshal(*template.Custom, &fields); err != nil {
		t.Fatal(err)
	}
	fields["platform"] = "linux-amd64"
	programPath := "ardents/linux-amd64/endpoint"
	generationPath := "ardents/linux-amd64/protected-endpoint"
	programCustom, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	programRaw := json.RawMessage(programCustom)
	targets.Signed.Targets = map[string]*metadata.TargetFiles{programPath: {Length: template.Length, Hashes: template.Hashes, Path: programPath, Custom: &programRaw}}
	// These are independently assembled signed target bytes. Resource parsing
	// and coherent installation remain with Enrollment/Installation.
	resources := map[string]string{}
	for _, name := range []string{"ardents-linux-amd64", "ardents-text-linux-amd64", "ardents-text-reader@.service", "ardents-text-publisher@.service", "ardents-text-reader@.socket", "ardents-text-publisher@.socket", "50-ardents-text.rules", "ardents-text.conf", "ardents-endpoint.service"} {
		b := []byte("test-resource:" + name)
		if name == "ardents-linux-amd64" {
			b = h.input.Artifact
		}
		d := sha256.Sum256(b)
		resources[name] = hex.EncodeToString(d[:])
	}
	generation, err := json.Marshal(struct {
		Schema   string            `json:"schema"`
		Platform string            `json:"platform"`
		Release  string            `json:"release_identity"`
		Version  int64             `json:"release_version"`
		Files    map[string]string `json:"files"`
	}{"ardents-protected-endpoint-artifact-v1", "linux-amd64", fields["release_identity"].(string), 1, resources})
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(generation)
	attestations := fields["builder_attestations"].([]any)
	for _, a := range attestations {
		a.(map[string]any)["target_sha256"] = hex.EncodeToString(digest[:])
	}
	genCustom, err := json.Marshal(fields)
	if err != nil {
		t.Fatal(err)
	}
	genRaw := json.RawMessage(genCustom)
	targets.Signed.Targets[generationPath] = &metadata.TargetFiles{Length: int64(len(generation)), Hashes: metadata.Hashes{"sha256": digest[:]}, Path: generationPath, Custom: &genRaw}
	h.input.Files[metadataBaseURL+"1.targets.json"] = historyBytes(t, targets)
	h.resignOnline(t, h.keys, 1)
	h.input.Local.Platform = "linux-amd64"
	h.input.TargetPath = programPath
	v, err := Open(filepath.Join(t.TempDir(), "history"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := v.Close(); err != nil {
			t.Error(err)
		}
	})
	program := v.Evaluate(context.Background(), h.input)
	programProof, ok := program.Authorization()
	if !ok {
		t.Fatalf("program refused %s %v", program.Outcome, program.Err())
	}
	h.input.TargetPath = generationPath
	h.input.Artifact = generation
	second := v.Evaluate(context.Background(), h.input)
	generationProof, ok := second.Authorization()
	if !ok || second.Outcome != OutcomeNoUpdate {
		t.Fatalf("fresh second target refused %s %v", second.Outcome, second.Err())
	}
	p, _ := programProof.AcceptedDecision()
	g, _ := generationProof.AcceptedDecision()
	if p.Path != programPath || g.Path != generationPath || p.Platform != g.Platform || p.ReleaseIdentity != g.ReleaseIdentity || p.ReleaseVersion != g.ReleaseVersion || !p.ReferenceTime.Equal(g.ReferenceTime) || p.Floors.TargetsVersion != g.Floors.TargetsVersion || !bytes.Equal(p.Floors.TargetsDigest, g.Floors.TargetsDigest) {
		t.Fatal("authorizations are not one exact signed set")
	}
	h.input.Artifact = append([]byte(nil), generation...)
	h.input.Artifact[len(h.input.Artifact)-1] ^= 1
	refused := v.Evaluate(context.Background(), h.input)
	if _, ok := refused.Authorization(); ok {
		t.Fatal("substituted generation authorized")
	}
	if refused.Outcome != OutcomeReleaseInvalid {
		t.Fatalf("substitution outcome %s", refused.Outcome)
	}
}
