package cli

import (
	"io"

	"ardents/internal/cli/output"
)

var groupDescriptions = map[string]string{
	"node":        "node lifecycle, runtime status and events",
	"network":     "network, discovery, peers and routes",
	"workload":    "workload lifecycle and hosted services",
	"data":        "objects, blobs, manifests and transfers",
	"diagnostics": "health, failures, pending operations and events",
	"config":      "effective operator configuration and atomic reload",
	"shell":       "interactive terminal session over the current operator context",
	"tui":         "optional fullscreen operator dashboard",
	"version":     "binary version, commit, build date and target platform",
}

func renderRootUsage(w io.Writer) {
	output.Writeln(w, "Usage: ard [global flags] <command> [subcommand]")
	output.Writeln(w)
	output.Writeln(w, "Commands:")
	for _, name := range []string{"node", "network", "workload", "data", "diagnostics", "config", "shell", "tui", "version"} {
		output.Writef(w, "  %-11s %s\n", name, groupDescriptions[name])
	}
	output.Writeln(w)
	output.Writeln(w, "Global flags:")
	for _, line := range []string{
		"  --addr         local API address",
		"  --token        bearer token override",
		"  --token-file   path to bearer token file",
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

func renderGroupUsage(w io.Writer, group string) {
	output.Writef(w, "Usage: ard [global flags] %s <subcommand>\n", group)
	output.Writeln(w, groupDescriptions[group])
}
