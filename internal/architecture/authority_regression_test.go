package architecture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAlphaCorpusDiagnosticHasNoPersistentFloorAuthority(t *testing.T) {
	root := repositoryRoot(t)
	command := string(readProjectFile(t, root, "cmd/ardents-control/main.go"))
	start := strings.Index(command, "func inspectAlphaCorpus(")
	if start < 0 {
		t.Fatal("cannot isolate inspect-alpha-corpus implementation")
	}
	end := strings.Index(command[start:], "\nfunc ")
	if end < 0 {
		t.Fatal("cannot isolate inspect-alpha-corpus implementation")
	}
	diagnostic := command[start : start+end]
	for _, forbidden := range []string{"state-root", "OpenPersistentFloor", ".Observe("} {
		if strings.Contains(diagnostic, forbidden) {
			t.Errorf("inspect-alpha-corpus retains floor authority %q", forbidden)
		}
	}
	acceptance := string(readProjectFile(t, root, "cmd/ardents-control/alpha_corpus_bundle.go"))
	for _, required := range []string{"func acceptAlphaCorpus(", "alpha.OpenPersistentFloor(", "floor.Observe("} {
		if !strings.Contains(acceptance, required) {
			t.Errorf("accept-alpha-corpus lacks sole floor mutation seam %q", required)
		}
	}
}

func TestTransitIssuerRootCustodyIsNotExportedFromDuty(t *testing.T) {
	root := repositoryRoot(t)
	exported := exportedPackageDeclarations(t, filepath.Join(root, "internal", "network", "duty"))
	for _, name := range []string{"InitializeTransitGrantIssuerRoot", "TransitGrantIssuerRoot", "InitializeTransitGrantIssuer"} {
		if exported[name] {
			t.Errorf("internal/network/duty still exports issuer-root custody %s", name)
		}
	}
}

func TestEndpointCompositionIsOwnedOnlyByParticipantRuntime(t *testing.T) {
	root := repositoryRoot(t)
	exported := exportedPackageDeclarations(t, filepath.Join(root, "internal", "endpoint"))
	for _, name := range []string{
		"New", "Setup", "OpenConnectionInterface", "ConnectionInterfaceConfig", "ApplicationStateView", "ApplicationEntry",
		"OpenUserApplicationConnection", "UserApplicationConnectionRequest", "OpenUserIntroductionRoute", "UserIntroductionRouteRequest",
		"OpenUserReachabilityRoute", "UserReachabilityRouteRequest", "ResolveUserReachability", "UserPrivateReachabilityRequest",
		"Connect", "Accept", "OutboundConnectionRequest", "InboundConnectionRequest",
	} {
		if exported[name] {
			t.Errorf("internal/endpoint still exports caller-authorized composition surface %s", name)
		}
	}
}

func exportedPackageDeclarations(t *testing.T, directory string) map[string]bool {
	t.Helper()
	result := make(map[string]bool)
	set := token.NewFileSet()
	packages, err := parser.ParseDir(set, directory, func(info os.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, parsed := range packages {
		for _, file := range parsed.Files {
			for _, declaration := range file.Decls {
				switch value := declaration.(type) {
				case *ast.FuncDecl:
					if ast.IsExported(value.Name.Name) {
						result[value.Name.Name] = true
					}
				case *ast.GenDecl:
					for _, spec := range value.Specs {
						if named, ok := spec.(*ast.TypeSpec); ok && ast.IsExported(named.Name.Name) {
							result[named.Name.Name] = true
						}
					}
				}
			}
		}
	}
	return result
}
