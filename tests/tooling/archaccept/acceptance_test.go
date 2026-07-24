package archaccept

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAcceptsDeclaredArchitecture(t *testing.T) {
	root := architectureFixture(t)

	require.NoError(t, Validate(root))
}

func TestValidateRejectsUndeclaredFileBudgetGrowth(t *testing.T) {
	root := architectureFixture(t)
	for i := 3; i <= 13; i++ {
		writeFixture(t, root, filepath.Join("internal/small", fmt.Sprintf("file_%02d.go", i)), "package small\n")
	}

	err := Validate(root)
	require.ErrorContains(t, err, "file budget")
	require.ErrorContains(t, err, "internal/small")
}

func TestValidateRejectsLooseFileBudgetException(t *testing.T) {
	root := architectureFixture(t)
	for i := 1; i <= 12; i++ {
		writeFixture(t, root, filepath.Join("internal/large", fmt.Sprintf("file_%02d.go", i)), "package large\n")
	}
	writeFixture(t, root, "internal/large/doc.go", "// Package large owns the large fixture. It does not own unrelated fixtures.\npackage large\n")
	updateManifest(t, root, func(manifest *Manifest) {
		manifest.FileBudget.Exceptions["internal/large"] = BudgetException{
			Max:    14,
			Reason: "fixture exception",
			Owner:  "ARD-023",
		}
	})

	err := Validate(root)
	require.ErrorContains(t, err, "exact ceiling")
	require.ErrorContains(t, err, "internal/large")
}

func TestValidateRejectsChangedDefaultFileBudget(t *testing.T) {
	root := architectureFixture(t)
	updateManifest(t, root, func(manifest *Manifest) {
		manifest.FileBudget.DefaultMax = 13
	})

	err := Validate(root)
	require.ErrorContains(t, err, "default_max must be 12")
}

func TestValidateChecksEveryDeclaredProductionRoot(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "sdk/go/newowner/owner.go", "package newowner\n")

	err := Validate(root)
	require.ErrorContains(t, err, "package documentation")
	require.ErrorContains(t, err, "sdk/go/newowner")
}

func TestValidateRejectsOmittedProductionRoot(t *testing.T) {
	root := architectureFixture(t)
	updateManifest(t, root, func(manifest *Manifest) {
		manifest.ProductionRoots = []string{"internal"}
	})

	err := Validate(root)
	require.ErrorContains(t, err, "production_roots must be exactly")
}

func TestValidateRejectsUndocumentedNewPackage(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/newowner/owner.go", "package newowner\n")

	err := Validate(root)
	require.ErrorContains(t, err, "package documentation")
	require.ErrorContains(t, err, "internal/newowner")
}

func TestValidateRejectsPackageCommentWithoutOwnershipBoundary(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/small/doc.go", "// Package small contains code.\npackage small\n")

	err := Validate(root)
	require.ErrorContains(t, err, "package documentation")
	require.ErrorContains(t, err, "responsibility and explicit non-responsibility")
}

func TestValidateRejectsPackageCommentWithOnlyNonResponsibility(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/small/doc.go", "// Package small does not own unrelated behavior.\npackage small\n")

	err := Validate(root)
	require.ErrorContains(t, err, "package documentation")
	require.ErrorContains(t, err, "responsibility and explicit non-responsibility")
}

func TestValidateRejectsServiceAndCompositionDrift(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "api/operator.proto", `
syntax = "proto3";
service NodeService {}
service SurpriseService {}
`)

	err := Validate(root)
	require.ErrorContains(t, err, "service contract")
	require.ErrorContains(t, err, "SurpriseService")
}

func TestValidateRejectsServiceRegistrationMentionedOnlyInComment(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/localapi/server_base.go", "package localapi\n// NewNodeServiceHandler\n")

	err := Validate(root)
	require.ErrorContains(t, err, "does not register NodeService")
}

func TestValidateRejectsServiceCallOutsideDeclaredCompositionFunction(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/localapi/server_base.go", "package localapi\nfunc compose() {}\nfunc dead() { register(NewNodeServiceHandler()) }\n")

	err := Validate(root)
	require.ErrorContains(t, err, "does not register NodeService")
}

func TestValidateRejectsAgentToolingOutsideAllowlist(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, ".agents/other/unsafe.md", "not approved\n")

	err := Validate(root)
	require.ErrorContains(t, err, "agent tooling")
	require.ErrorContains(t, err, ".agents/other/unsafe.md")
}

func TestValidateRejectsBroadenedAgentToolingAllowlist(t *testing.T) {
	root := t.TempDir()
	writeAgentToolingFixture(t, root)

	err := errors.Join(validateAgentTooling(root, AgentToolingPolicy{
		Root:            ".agents",
		AllowedPrefixes: []string{".agents/"},
	})...)
	require.ErrorContains(t, err, "agent tooling policy must allow exactly")
}

func TestValidateRejectsMissingAgentToolingLock(t *testing.T) {
	root := t.TempDir()
	writeAgentToolingFixture(t, root)
	require.NoError(t, os.Remove(filepath.Join(root, "skills-lock.json")))

	err := errors.Join(validateAgentTooling(root, AgentToolingPolicy{
		Root:            requiredAgentToolingRoot,
		AllowedPrefixes: []string{requiredAgentToolingPrefix},
	})...)
	require.ErrorContains(t, err, "agent tooling skills lock")
}

func TestValidateRejectsMissingAgentToolingGovernanceDecision(t *testing.T) {
	root := t.TempDir()
	writeAgentToolingFixture(t, root)

	err := errors.Join(validateAgentTooling(root, AgentToolingPolicy{
		Root:            requiredAgentToolingRoot,
		AllowedPrefixes: []string{requiredAgentToolingPrefix},
	})...)
	require.ErrorContains(t, err, "agent tooling governance decision")
}

func TestValidateRejectsAgentToolingContentDrift(t *testing.T) {
	root := t.TempDir()
	writeAgentToolingFixture(t, root)
	writeFixture(t, root, requiredAgentToolingDecision, "# ADR 0010: Repository-local agent tooling\n\n- Status: Accepted\n\nKeep `.agents/skills/security-audit/` pinned by `skills-lock.json`.\n")
	writeFixture(t, root, ".agents/skills/security-audit/SKILL.md", "changed without updating the lock\n")

	err := errors.Join(validateAgentTooling(root, AgentToolingPolicy{
		Root:            requiredAgentToolingRoot,
		AllowedPrefixes: []string{requiredAgentToolingPrefix},
	})...)
	require.ErrorContains(t, err, "content hash does not match")
}

func TestValidateRejectsAgentToolingSymlink(t *testing.T) {
	root := t.TempDir()
	writeAgentToolingFixture(t, root)
	writeFixture(t, root, requiredAgentToolingDecision, "# ADR 0010: Repository-local agent tooling\n\n- Status: Accepted\n\nKeep `.agents/skills/security-audit/` pinned by `skills-lock.json`.\n")
	link := filepath.Join(root, ".agents", "skills", "security-audit", "linked.md")
	if err := os.Symlink("SKILL.md", link); err != nil {
		t.Skipf("symlink creation is unavailable: %v", err)
	}

	err := errors.Join(validateAgentTooling(root, AgentToolingPolicy{
		Root:            requiredAgentToolingRoot,
		AllowedPrefixes: []string{requiredAgentToolingPrefix},
	})...)
	require.ErrorContains(t, err, "unsupported non-regular skill entry")
}

func TestValidateRejectsPrivateProtocolBoundaryDrift(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/messaging/protocol/private.pb.go", "// source: elsewhere/private.proto\npackage messagingprotocol\n")

	err := Validate(root)
	require.ErrorContains(t, err, "private protocol")
	require.ErrorContains(t, err, "generated source")
}

func TestValidateRejectsSecondPrivateGeneratedOutput(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/elsewhere/private_copy.pb.go", "// source: api/ardents/private/v1/private.proto\npackage elsewhere\n")

	err := Validate(root)
	require.ErrorContains(t, err, "private protocol")
	require.ErrorContains(t, err, "private_copy.pb.go")
}

func TestValidateRejectsLegacyPrivateProtocolLocation(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "internal/messaging/private.proto", "syntax = \"proto3\";\n")

	err := Validate(root)
	require.ErrorContains(t, err, "private protocol")
	require.ErrorContains(t, err, "legacy location")
}

func TestValidateRejectsPrivateProtocolGeneratorDrift(t *testing.T) {
	root := architectureFixture(t)
	writeFixture(t, root, "scripts/generate-api.ps1", "Write-Host 'private protocol generation removed'\n")

	err := Validate(root)
	require.ErrorContains(t, err, "private protocol")
	require.ErrorContains(t, err, "generator")
}

func TestRepositoryArchitectureAcceptance(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	require.NoError(t, Validate(root))
}

func architectureFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	for _, productionRoot := range requiredProductionRoots {
		require.NoError(t, os.MkdirAll(filepath.Join(root, filepath.FromSlash(productionRoot)), 0o755))
	}
	writeFixture(t, root, "internal/small/doc.go", "// Package small owns the fixture. It does not own unrelated fixture behavior.\npackage small\n")
	writeFixture(t, root, "internal/small/owner.go", "package small\n")
	writeFixture(t, root, "internal/legacy/owner.go", "package legacy\n")
	writeAgentToolingFixture(t, root)
	writeFixture(t, root, requiredAgentToolingDecision, "# ADR 0010: Repository-local agent tooling\n\n- Status: Accepted\n\nKeep `.agents/skills/security-audit/` pinned by `skills-lock.json`.\n")
	writeFixture(t, root, "api/operator.proto", "syntax = \"proto3\";\nservice NodeService {}\n")
	writeFixture(t, root, "api/application.proto", "syntax = \"proto3\";\nservice ContentService {}\n")
	writeFixture(t, root, "internal/localapi/server_base.go", "package localapi\nfunc compose() { register(NewNodeServiceHandler()) }\n")
	writeFixture(t, root, "internal/applicationapi/content/handler.go", "package content\nfunc compose() (string, Handler, error) { path, handler := NewContentServiceHandler(); return path, handler, nil }\n")
	writeFixture(t, root, "scripts/generate-api.ps1", `$privateProtocolSource = "api/ardents/private/v1/private.proto"
& protoc --proto_path=$root --go_out=$outputRoot --go_opt=module=ardents $privateProtocolSource
`)
	writeFixture(t, root, "api/ardents/private/v1/private.proto", `
syntax = "proto3";
package ardents.private.v1;
option go_package = "ardents/internal/messaging/protocol;messagingprotocol";
`)
	writeFixture(t, root, "internal/messaging/protocol/private.pb.go", "// source: api/ardents/private/v1/private.proto\npackage messagingprotocol\n")

	manifest := Manifest{
		Version:         1,
		ProductionRoots: append([]string(nil), requiredProductionRoots...),
		FileBudget: FileBudgetPolicy{
			DefaultMax: 12,
			Exceptions: map[string]BudgetException{},
		},
		PackageDocumentation: PackageDocumentationPolicy{
			Grandfathered: map[string]string{
				"internal/legacy":                 "existing package; documentation tracked by ARD-029",
				"internal/localapi":               "composition fixture",
				"internal/applicationapi/content": "composition fixture",
			},
		},
		Services: ServicePolicy{
			Operator: ServiceSurface{
				ProtoRoots: []string{"api/operator.proto"},
				Composition: map[string]CompositionTarget{
					"NodeService": {
						Path:     "internal/localapi/server_base.go",
						Function: "compose",
						Mode:     "register_argument",
					},
				},
			},
			Application: ServiceSurface{
				ProtoRoots: []string{"api/application.proto"},
				Composition: map[string]CompositionTarget{
					"ContentService": {
						Path:     "internal/applicationapi/content/handler.go",
						Function: "compose",
						Mode:     "returned_handler",
					},
				},
			},
		},
		AgentTooling: AgentToolingPolicy{
			Root:            ".agents",
			AllowedPrefixes: []string{".agents/skills/security-audit/"},
		},
		PrivateProtocol: PrivateProtocolPolicy{
			Proto:     canonicalPrivateProto,
			Generated: canonicalPrivateGenerated,
			GoPackage: canonicalPrivateGoPackage,
			Generator: canonicalPrivateGenerator,
			Owner:     "ARD-028",
		},
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	writeFixture(t, root, "docs/engineering/architecture-acceptance.json", string(raw))
	return root
}

func writeAgentToolingFixture(t *testing.T, root string) {
	t.Helper()

	writeFixture(t, root, ".agents/skills/security-audit/SKILL.md", "approved\n")
	writeFixture(t, root, "skills-lock.json", `{
  "version": 1,
  "skills": {
    "security-audit": {
      "source": "cloudflare/security-audit-skill",
      "sourceType": "github",
      "skillPath": "skills/security-audit/SKILL.md",
      "computedHash": "869eb59b69d5eaf138cd6600b5579ae41a6f7c9d703a00736b349cf564a09fd6"
    }
  }
}`)
}

func writeFixture(t *testing.T, root string, relativePath string, contents string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}

func updateManifest(t *testing.T, root string, update func(*Manifest)) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(manifestPath))
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	var manifest Manifest
	require.NoError(t, json.Unmarshal(raw, &manifest))
	update(&manifest)
	raw, err = json.MarshalIndent(manifest, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, raw, 0o644))
}
