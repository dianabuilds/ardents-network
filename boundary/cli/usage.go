package cli

import (
	"fmt"
	"io"
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
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] <command> [subcommand]")
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Commands:")
	for _, name := range []string{"node", "network", "workload", "data", "diagnostics", "config", "shell", "tui", "version"} {
		_, _ = fmt.Fprintf(w, "  %-11s %s\n", name, groupDescriptions[name])
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, "Global flags:")
	_, _ = fmt.Fprintln(w, "  --addr         local API address")
	_, _ = fmt.Fprintln(w, "  --token        bearer token override")
	_, _ = fmt.Fprintln(w, "  --token-file   path to bearer token file")
	_, _ = fmt.Fprintln(w, "  --context      named operator context")
	_, _ = fmt.Fprintln(w, "  --context-file path to contexts file")
	_, _ = fmt.Fprintln(w, "  --principal    expected node principal for identity preflight")
	_, _ = fmt.Fprintln(w, "  --public-key   expected node public key for identity preflight")
	_, _ = fmt.Fprintln(w, "  --scope        scope hint for operator help and preflight")
	_, _ = fmt.Fprintln(w, "  --output       output mode: human or json")
	_, _ = fmt.Fprintln(w, "  --watch        enable watch mode when supported")
	_, _ = fmt.Fprintln(w, "  --interval     polling interval for watch mode")
	_, _ = fmt.Fprintln(w, "  --timeout      request timeout")
}

func renderGroupUsage(w io.Writer, group string) {
	_, _ = fmt.Fprintf(w, "Usage: ard [global flags] %s <subcommand>\n", group)
	_, _ = fmt.Fprintf(w, "%s\n", groupDescriptions[group])
}

func renderNodeUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] node <start|stop|status|runtime|capabilities|events>")
}

func renderNetworkUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] network <status|discovery|presence|peers|routes|resolve|records>")
}

func renderNetworkResolveUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] network resolve <record|service> [flags]")
}

func renderNetworkRecordsUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] network records <list|import> [flags]")
}

func renderDiagnosticsUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] diagnostics <snapshot|health|pending|explain|events>")
}

func renderConfigUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] config <show|reload>")
}

func renderWorkloadUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] workload <list|get|register|start|stop|restart|services|service|publication>")
}

func renderDataUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] data <inventory|objects|blobs|manifests|transfers>")
}

func renderTUIUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] tui")
}

func renderShellUsage(w io.Writer) {
	_, _ = fmt.Fprintln(w, "Usage: ard [global flags] shell")
}
