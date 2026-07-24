package doccontract

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateAcceptsPrincipalOnlyActiveDocumentation(t *testing.T) {
	root := canonicalFixture(t)

	require.NoError(t, Validate(root))
}

func TestValidateRejectsLegacyOperatorDirectives(t *testing.T) {
	cases := []string{
		"Use the token-authenticated loopback surface.",
		"Create an API token during bootstrap.",
		"Connect to the daemon's loopback control API.",
		"Rotate the API and observability token.",
		"Set the server-side token from an environment value.",
		"Use the loopback-only authenticated operator API.",
		"Run ardentsctl --token-file ./operator-token node status.",
		"Authenticate operator control with a bearer token over localhost.",
	}

	for _, directive := range cases {
		t.Run(directive, func(t *testing.T) {
			root := canonicalFixture(t)
			writeFixture(t, root, "README.md", directive)

			err := Validate(root)
			require.Error(t, err)
			require.Contains(t, strings.ToLower(err.Error()), "legacy operator directive")
		})
	}
}

func TestValidateRejectsBrokenLocalLinksAndMissingScripts(t *testing.T) {
	root := canonicalFixture(t)
	writeFixture(t, root, "README.md", `
[missing contract](docs/operations/missing.md)

`+"```powershell"+`
./scripts/missing.ps1
`+"```"+`
`)

	err := Validate(root)
	require.Error(t, err)
	require.Contains(t, err.Error(), "docs/operations/missing.md")
	require.Contains(t, err.Error(), "scripts/missing.ps1")
}

func TestValidateScansAllActiveDocumentationButExcludesAuditHistory(t *testing.T) {
	root := canonicalFixture(t)
	writeFixture(t, root, "docs/operations/another-runbook.md", `
Use the token-authenticated loopback surface.
[missing active link](missing.md)
`)
	writeFixture(t, root, "docs/audit/old-review.md", "Use the token-authenticated loopback surface.\n")

	err := Validate(root)
	require.Error(t, err)
	require.Contains(t, err.Error(), "docs/operations/another-runbook.md")
	require.Contains(t, err.Error(), "missing.md")
	require.NotContains(t, err.Error(), "docs/audit/old-review.md")
}

func TestValidateRejectsUnknownDocumentedCLICommand(t *testing.T) {
	root := canonicalFixture(t)
	writeFixture(t, root, "README.md", `
`+"```sh"+`
ardentsctl frobnicate node
`+"```"+`
`)

	err := Validate(root)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown ardentsctl command")
}

func TestValidateUsesProductionCatalogueForNestedHelp(t *testing.T) {
	root := canonicalFixture(t)
	writeFixture(t, root, "README.md", `
`+"```sh"+`
ardentsctl network resolve help
ardentsctl data objects help
ardentsctl identity application-ticket help
ardentsctl shell help
`+"```"+`
`)

	require.NoError(t, Validate(root))
}

func TestValidateAllowsExplicitLegacyRejection(t *testing.T) {
	root := canonicalFixture(t)
	writeFixture(t, root, "docs/operations/operator-access-contract.md", `
The Operator Interface does not accept bearer tokens over loopback TCP.
The token-authenticated loopback surface is unsupported.
`)

	require.NoError(t, Validate(root))
}

func TestValidateDoesNotLetEarlierNegationHideLegacyDirective(t *testing.T) {
	root := canonicalFixture(t)
	writeFixture(t, root, "README.md", "This transport is not supported. Use the token-authenticated loopback surface.\n")

	err := Validate(root)
	require.Error(t, err)
	require.Contains(t, err.Error(), "token-authenticated loopback")
}

func TestValidateDoesNotTreatFallbackConditionsAsLegacyRejection(t *testing.T) {
	for _, directive := range []string{
		"If the Unix socket is not available, use the token-authenticated loopback control API.",
		"Use the loopback control API without TLS.",
		"Use the legacy token-authenticated loopback control API.",
		"If the Unix socket is unsupported, use the token-authenticated loopback control API.",
	} {
		t.Run(directive, func(t *testing.T) {
			root := canonicalFixture(t)
			writeFixture(t, root, "README.md", directive)

			err := Validate(root)
			require.Error(t, err)
			require.Contains(t, strings.ToLower(err.Error()), "legacy operator directive")
		})
	}
}

func TestValidateRejectsUnknownDockerAndGoCommands(t *testing.T) {
	for _, command := range []string{
		"docker compose frobnicate",
		"go frobnicate ./...",
	} {
		t.Run(command, func(t *testing.T) {
			root := canonicalFixture(t)
			writeFixture(t, root, "README.md", "```sh\n"+command+"\n```\n")

			err := Validate(root)
			require.Error(t, err)
			require.Contains(t, err.Error(), "unknown documented command")
		})
	}
}

func TestRepositoryDocumentationContract(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	require.NoError(t, err)
	require.NoError(t, Validate(root))
}

func canonicalFixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeFixture(t, root, "README.md", `
[operator contract](docs/operations/operator-access-contract.md)

`+"```powershell"+`
./ardents.ps1 status
`+"```"+`
`)
	writeFixture(t, root, "docs/operations/operator-access-contract.md", "# Operator access\n")
	writeFixture(t, root, "docs/product/distribution-model.md", "The protected Operator Interface uses a Unix socket.\n")
	writeFixture(t, root, "docs/protocols/communication-contracts.md", "Principal sessions authenticate protected calls.\n")
	writeFixture(t, root, "docs/operations/operator-runbook.md", "Rotate device Credentials and Access Grants deliberately.\n")
	writeFixture(t, root, "ardents.ps1", "# fixture\n")
	return root
}

func writeFixture(t *testing.T, root string, relativePath string, contents string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(relativePath))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o644))
}
