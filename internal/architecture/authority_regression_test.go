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

func TestLifecycleBypassPrimitivesAreNotExported(t *testing.T) {
	root := repositoryRoot(t)
	entryExports := exportedPackageDeclarations(t, filepath.Join(root, "internal", "entry"))
	for _, name := range []string{"Admit", "Consume"} {
		if entryExports[name] {
			t.Errorf("internal/entry still exports split admission primitive %s", name)
		}
	}
	nodeExports := exportedPackageDeclarations(t, filepath.Join(root, "internal", "node"))
	for _, name := range []string{
		"StartInitiator", "StartRendezvous", "StartIntroduction", "StartResponder",
		"InitiatorConfig", "RendezvousConfig", "IntroductionConfig", "ResponderConfig",
		"Initiator", "Rendezvous", "Introduction", "Responder",
		"InitiatorUsage", "RendezvousUsage", "IntroductionUsage", "ResponderUsage",
		"InitiatorPeer", "RendezvousPeer", "ResponderPeer", "CredentialIssuer", "ResolutionGateway",
	} {
		if nodeExports[name] {
			t.Errorf("internal/node still exports lifecycle-bypass primitive %s", name)
		}
	}
	if exportedStructField(t, filepath.Join(root, "internal", "route"), "Attachment", "Connection") {
		t.Error("internal/route Attachment still exports its raw Connection")
	}
}

func TestMaintainedTruthDoesNotClaimUnownedCompositionOrUnprovenCleanup(t *testing.T) {
	root := repositoryRoot(t)
	checks := []struct {
		path      string
		forbidden []string
	}{
		{"docs/technical/enrollment-verification.md", []string{"Browser Entry companions", "cmd/ardents-browser-entry` exercise the same narrow interface"}},
		{"docs/technical/naming.md", []string{"production Gateway or Resolver path", "Production Resolution consumes"}},
		{"docs/technical/network-route-node.md", []string{"listener's start snapshot identifies"}},
		{"internal/route/route.go", []string{"returns one atomic current fact", "Once Close returns, no Route selection or resource"}},
		{"internal/route/native_attachment.go", []string{"Close releases the authenticated Entry attempt"}},
		{"internal/node/lifecycle.go", []string{"returns only after terminal cleanup"}},
		{"internal/node/initiator_relay.go", []string{"joins all duty-owned work before"}},
		{"internal/node/introduction_listener.go", []string{"joins all TLS/control work"}},
	}
	for _, check := range checks {
		content := string(readProjectFile(t, root, check.path))
		for _, forbidden := range check.forbidden {
			if strings.Contains(content, forbidden) {
				t.Errorf("%s retains overstated truth %q", check.path, forbidden)
			}
		}
	}
}

func exportedStructField(t *testing.T, directory, typeName, fieldName string) bool {
	t.Helper()
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
				generated, ok := declaration.(*ast.GenDecl)
				if !ok {
					continue
				}
				for _, spec := range generated.Specs {
					named, ok := spec.(*ast.TypeSpec)
					if !ok || named.Name.Name != typeName {
						continue
					}
					structure, ok := named.Type.(*ast.StructType)
					if !ok {
						return false
					}
					for _, field := range structure.Fields.List {
						for _, name := range field.Names {
							if name.Name == fieldName && ast.IsExported(name.Name) {
								return true
							}
						}
					}
				}
			}
		}
	}
	return false
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
