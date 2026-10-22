package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

// Public flags prepare the existing fixed genesis declaration. No JSON editing,
// private key, assignment redraw, State acceptance or runtime work is needed.
func runSourceInitialPlan(args []string, out io.Writer) error {
	flags := flag.NewFlagSet("source prepare-initial-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	values := make(map[string]*string)
	for _, name := range []string{"network", "node", "node-key", "state-key", "family", "endpoint", "valid-from", "valid-until", "assignment-seed", "domains", "output"} {
		values[name] = flags.String(name, "", name)
	}
	if flags.Parse(args) != nil || flags.NArg() != 0 {
		return errSourceControlInput
	}
	p := sourceInitialNetworkPlan{Schema: "ardents-source-initial-network-plan-v1", Reference: time.Now().UTC().Truncate(time.Second), Family: *values["family"], Endpoint: *values["endpoint"], Domains: strings.Split(*values["domains"], ",")}
	for name, destination := range map[string]*sourceInitialID{"network": &p.Network, "node": &p.Node, "node-key": &p.NodeKey, "state-key": &p.StateKey, "assignment-seed": &p.Seed} {
		id, err := sourcePublicID(*values[name])
		if err != nil {
			return err
		}
		*destination = sourceInitialID(id)
	}
	var err error
	p.From, err = time.Parse(time.RFC3339, *values["valid-from"])
	if err != nil {
		return errSourceControlInput
	}
	p.Until, err = time.Parse(time.RFC3339, *values["valid-until"])
	if err != nil {
		return errSourceControlInput
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := prepareSourceInitialPlan(ctx, p, *values["output"], nil); err != nil {
		return &sourceControlFailure{err}
	}
	raw, err := json.Marshal(struct {
		Operation string `json:"operation"`
		StateKey  string `json:"state_public_key"`
		NodeKey   string `json:"node_public_key"`
		Accepted  bool   `json:"state_accepted"`
	}{"source.prepare-initial-plan", hex.EncodeToString(p.StateKey[:]), hex.EncodeToString(p.NodeKey[:]), false})
	if err != nil {
		return err
	}
	return writeSourceControlBytes(ctx, out, append(raw, '\n'))
}

func prepareSourceInitialPlan(ctx context.Context, p sourceInitialNetworkPlan, output string, observe func(string, *os.File)) error {
	raw, err := json.Marshal(p)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	check := func() error {
		// Epoch preparation later validates domains and assignment together with
		// the actual signed Record. Public declaration is no successful Epoch.
		_, err := decodeSourceInitialNetwork(raw, time.Now().UTC())
		return err
	}
	return persistSourceArtifacts(ctx, output, []sourceArtifactFile{{"source-initial-network.json", raw}}, check, observe)
}
