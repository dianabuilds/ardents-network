package cli

import (
	"io"
	"strings"

	"ardents/internal/cli/catalog"
	"ardents/internal/cli/output"
)

func renderRootUsage(w io.Writer) {
	output.Writeln(w, "Usage: ardentsctl [global flags] <command> [subcommand]")
	output.Writeln(w)
	output.Writeln(w, "Commands:")
	for _, group := range catalog.Groups() {
		output.Writef(w, "  %-11s %s\n", group.Name, group.Summary)
	}
	output.Writeln(w)
	output.Writeln(w, "Global flags:")
	for _, line := range []string{
		"  --addr         local API address",
		"  --ssh          OpenSSH target for secure remote access",
		"  --ssh-port     OpenSSH server port (default 22)",
		"  --ssh-identity OpenSSH private key path",
		"  --ssh-known-hosts OpenSSH known_hosts path",
		"  --ssh-operator-socket absolute remote Operator Unix socket",
		"  --signer-file  protected device signer bundle",
		"  --context      named operator context",
		"  --context-file path to contexts file",
		"  --principal    expected node principal for identity preflight",
		"  --public-key   expected node public key for identity preflight",
		"  --scope        scope hint for operator help and preflight",
		"  --output       output mode: human or json",
		"  --watch        enable watch mode when supported",
		"  --interval     polling interval for watch mode",
		"  --timeout      request timeout",
	} {
		output.Writeln(w, line)
	}
}

func renderCatalogueUsage(w io.Writer, prefix []string) bool {
	specs := catalog.Under(prefix)
	if len(specs) == 0 {
		return false
	}
	if spec, exact := catalog.Exact(prefix); exact && len(specs) == 1 {
		output.Writef(w, "Usage: ardentsctl [global flags] %s\n", spec.Usage)
		output.Writeln(w, spec.Summary)
		output.Writef(w, "Output: %s\n", spec.Output)
		return true
	}
	output.Writef(w, "Usage: ardentsctl [global flags] %s <subcommand>\n", strings.Join(prefix, " "))
	if len(prefix) == 1 {
		for _, group := range catalog.Groups() {
			if group.Name == prefix[0] {
				output.Writeln(w, group.Summary)
				break
			}
		}
	}
	output.Writeln(w)
	output.Writeln(w, "Commands:")
	for _, spec := range specs {
		output.Writef(w, "  %-72s %s\n", spec.Usage, spec.Summary)
	}
	return true
}
