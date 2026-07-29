package topology

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	configurationcmd "ardents/internal/cli/configuration"
	"ardents/internal/cli/output"
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
		if err := output.JSONValue(command.Out, status); err != nil {
			fmt.Fprintln(command.Err, "ardentsctl: topology status output failed")
			return 1
		}
	} else {
		if err := renderHuman(command.Out, status); err != nil {
			fmt.Fprintln(command.Err, "ardentsctl: topology status output failed")
			return 1
		}
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

func renderHuman(out io.Writer, status deployment.TopologyStatus) error {
	if _, err := fmt.Fprintf(out, "topology: %s\n", status.Outcome); err != nil {
		return err
	}
	rows := make([][]string, 0, len(status.Nodes))
	for _, node := range status.Nodes {
		rows = append(rows, []string{
			node.Slot, node.Role, string(node.Observation), fmt.Sprint(node.Ready),
			string(node.Readiness), fmt.Sprint(node.Joined), string(node.Reachability),
			string(node.Store), string(node.Image), string(node.Reason),
		})
	}
	return output.Table(out,
		[]string{"NODE", "ROLE", "OBSERVATION", "READY", "READINESS", "JOINED", "REACHABILITY", "STORE", "IMAGE", "REASON"},
		rows,
	)
}
