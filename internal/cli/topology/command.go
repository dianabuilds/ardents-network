package topology

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	configurationcmd "ardents/internal/cli/configuration"
	"ardents/internal/deployment"
)

const maxManifestBytes = 256 << 10

// Command runs the bounded MR-02 aggregate without creating a shared client.
type Command struct {
	Base    configurationcmd.Config
	Out     io.Writer
	Err     io.Writer
	Factory clientFactory
}

func (command Command) Run(ctx context.Context, args []string) int {
	if len(args) == 0 || args[0] != "status" {
		fmt.Fprintln(command.Err, "usage: topology status --manifest FILE")
		return 2
	}
	flags := flag.NewFlagSet("topology status", flag.ContinueOnError)
	flags.SetOutput(command.Err)
	var manifestPath string
	flags.StringVar(&manifestPath, "manifest", "", "reviewed topology manifest")
	if err := flags.Parse(args[1:]); err != nil || manifestPath == "" || flags.NArg() != 0 {
		if err == nil {
			fmt.Fprintln(command.Err, "usage: topology status --manifest FILE")
		}
		return 2
	}
	raw, err := readBoundedFile(manifestPath)
	if err != nil {
		fmt.Fprintln(command.Err, "ardentsctl: topology manifest is unavailable")
		return 2
	}
	status, err := (deployment.StatusInspector{
		Probe: Probe{Base: command.Base, Factory: command.Factory},
	}).Inspect(ctx, raw)
	if err != nil {
		fmt.Fprintf(command.Err, "ardentsctl: invalid topology manifest: %v\n", err)
		return 2
	}
	if command.Base.Output == "json" {
		encoded, marshalErr := json.MarshalIndent(status, "", "  ")
		if marshalErr != nil {
			fmt.Fprintln(command.Err, "ardentsctl: topology status output failed")
			return 1
		}
		fmt.Fprintln(command.Out, string(encoded))
	} else {
		renderHuman(command.Out, status)
	}
	if status.Outcome == deployment.TopologyOutcomeReady {
		return 0
	}
	return 1
}

func readBoundedFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("manifest must be a regular file")
	}
	raw, err := io.ReadAll(io.LimitReader(file, maxManifestBytes+1))
	if err != nil || len(raw) > maxManifestBytes {
		return nil, fmt.Errorf("manifest exceeds size limit")
	}
	return raw, nil
}

func renderHuman(out io.Writer, status deployment.TopologyStatus) {
	fmt.Fprintf(out, "topology: %s\n", status.Outcome)
	table := tabwriter.NewWriter(out, 0, 4, 2, ' ', 0)
	fmt.Fprintln(table, "NODE\tROLE\tOBSERVATION\tREADY\tREACHABILITY\tSTORE\tIMAGE\tREASON")
	for _, node := range status.Nodes {
		fmt.Fprintf(table, "%s\t%s\t%s\t%t\t%s\t%s\t%s\t%s\n",
			node.Slot, node.Role, node.Observation, node.Ready,
			node.Reachability, node.Store, node.Image, node.Reason)
	}
	_ = table.Flush()
}
