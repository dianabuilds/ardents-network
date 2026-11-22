package main

import (
	"context"
	"flag"
	"io"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/admission/issuance"
)

// This is an offline key-material request, not an issuer appointment, signed
// profile, Permission or runtime plan. Admission owns generation and storage.
type forwardingTokenMaterialPlan struct {
	Schema  string    `json:"schema"`
	Network string    `json:"network_id"`
	Issuer  string    `json:"issuer_node_id"`
	Signer  string    `json:"issuer_node_key"`
	Start   time.Time `json:"start"`
	End     time.Time `json:"end"`
}

type forwardingTokenInventory struct {
	Schema    string             `json:"schema"`
	Inventory issuance.Inventory `json:"inventory"`
}

func forwardingTokenMaterialArguments(args []string) (p forwardingTokenMaterialPlan, binding issuance.Binding, output string, returned error) {
	if len(args) > 16 {
		return p, binding, output, errSourceControlInput
	}
	p.Schema = "ardents-forwarding-token-key-plan-v1"
	flags := flag.NewFlagSet("forwarding prepare-token-keys", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var start, end string
	flags.StringVar(&p.Network, "network-id", "", "explicit Network ID")
	flags.StringVar(&p.Issuer, "issuer-node-id", "", "explicit issuer Node ID")
	flags.StringVar(&p.Signer, "issuer-node-key", "", "explicit public issuer Node key")
	flags.StringVar(&start, "start", "", "first aligned UTC hour")
	flags.StringVar(&end, "end", "", "exclusive aligned UTC hour")
	flags.StringVar(&output, "output", "", "fresh protected offline material output")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 6 || !uniqueControlOptions(args, "") || !sourceControlPath(output) {
		return p, binding, output, errSourceControlInput
	}
	var err error
	for _, pair := range []struct {
		text   string
		target *[32]byte
	}{{p.Network, &binding.Network}, {p.Issuer, &binding.Issuer}, {p.Signer, &binding.Signer}} {
		*pair.target, err = controlPublic32(pair.text)
		if err != nil {
			return p, binding, output, err
		}
	}
	p.Start, err = time.Parse(time.RFC3339, start)
	if err != nil || p.Start.Location() != time.UTC || p.Start.Format(time.RFC3339) != start || p.Start.Unix() < 0 || p.Start.Unix()%3600 != 0 || p.Start.Nanosecond() != 0 {
		return p, binding, output, errSourceControlInput
	}
	p.End, err = time.Parse(time.RFC3339, end)
	if err != nil || p.End.Location() != time.UTC || p.End.Format(time.RFC3339) != end || p.End.Unix()%3600 != 0 || p.End.Nanosecond() != 0 || !p.End.After(p.Start) || p.End.Sub(p.Start) > 6*time.Hour {
		return p, binding, output, errSourceControlInput
	}
	binding.Start, binding.End = p.Start, p.End
	return p, binding, output, nil
}

// The Forwarding material purpose selects only its own plan and inventory
// schemas through the shared offline ceremony; Admission owns the material root.
func prepareForwardingTokenMaterial(ctx context.Context, args []string, observe func(string, *os.File)) ([]byte, error) {
	return prepareNodeTokenMaterial(ctx, args, observe, forwardingTokenMaterialArguments, func(inventory issuance.Inventory) forwardingTokenInventory {
		return forwardingTokenInventory{"ardents-forwarding-token-inventory-v1", inventory}
	})
}

func runForwardingTokenMaterial(args []string, out io.Writer) error {
	return runNodeTokenMaterial("forwarding", args, out, func(err error) error { return &forwardingControlFailure{err} }, prepareForwardingTokenMaterial)
}
