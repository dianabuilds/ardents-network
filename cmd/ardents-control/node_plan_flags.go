package main

import (
	"encoding/hex"
	"flag"
	"io"
	"strings"
)

type nodePlanAuthorities struct {
	values  []string
	invalid error
}

func (p *nodePlanAuthorities) String() string { return strings.Join(p.values, ",") }
func (p *nodePlanAuthorities) Set(value string) error {
	if len(p.values) >= 16 || len(value) != 64 {
		return p.invalid
	}
	p.values = append(p.values, value)
	return nil
}

type nodePlanFlagValues struct {
	requestPath, destination, networkID, nodeID, profileAuthority string
	pins                                                          nodePlanAuthorities
	wait                                                          uint64
}

// nodePlanFlags parses the shared public inputs; role code owns request
// matching and the distinct runtime declaration.
func nodePlanFlags(args []string, name string, invalid error, threshold *int, workTx, workRx, terminationTx, terminationRx *uint64) (nodePlanFlagValues, error) {
	var v nodePlanFlagValues
	v.pins.invalid = invalid
	if len(args) > 64 {
		return v, invalid
	}
	flags := flag.NewFlagSet(name+" prepare-plan", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.StringVar(&v.requestPath, "request", "", "original canonical request")
	flags.StringVar(&v.destination, "output", "", "fresh output directory")
	flags.StringVar(&v.networkID, "network-id", "", "exact Network ID")
	flags.StringVar(&v.nodeID, "node-id", "", "exact Node ID")
	flags.StringVar(&v.profileAuthority, "profile-authority", "", "independently approved profile key")
	flags.Var(&v.pins, "authority", "independently approved State key; at most sixteen")
	flags.IntVar(threshold, "threshold", 0, "State signature threshold")
	flags.Uint64Var(workTx, "work-tx", 0, "work Tx coverage")
	flags.Uint64Var(workRx, "work-rx", 0, "work Rx coverage")
	flags.Uint64Var(terminationTx, "termination-tx", 0, "termination Tx coverage")
	flags.Uint64Var(terminationRx, "termination-rx", 0, "termination Rx coverage")
	flags.Uint64Var(&v.wait, "retirement-wait-ms", 0, "bounded notice wait")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 12 || !uniqueNodeOptions(args, "authority") || !sourceControlPath(v.destination) || v.wait == 0 || v.wait > 5000 {
		return v, invalid
	}
	return v, nil
}

// Node preparation flag producers reject implicit last-wins selection.
// Only the plan's exact State-authority option is intentionally repeatable.
func uniqueNodeOptions(args []string, repeatable string) bool {
	seen := map[string]bool{}
	for _, argument := range args {
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		name := strings.TrimLeft(argument, "-")
		name, _, _ = strings.Cut(name, "=")
		if seen[name] && name != repeatable {
			return false
		}
		seen[name] = true
	}
	return true
}

// nodePlanKeys decodes declarations in their established order. It supplies no
// authenticated Network or Node authority.
func nodePlanKeys(network, node, profile string, pins []string, invalid error) ([32]byte, [32]byte, [32]byte, [][32]byte, error) {
	decode := func(value string) ([32]byte, error) {
		var result [32]byte
		raw, err := hex.DecodeString(value)
		if err != nil || len(raw) != 32 || hex.EncodeToString(raw) != value {
			return result, invalid
		}
		copy(result[:], raw)
		return result, nil
	}
	n, err := decode(network)
	if err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, nil, err
	}
	id, err := decode(node)
	if err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, nil, err
	}
	authority, err := decode(profile)
	if err != nil {
		return [32]byte{}, [32]byte{}, [32]byte{}, nil, err
	}
	var values [][32]byte
	for _, pin := range pins {
		key, err := decode(pin)
		if err != nil {
			return [32]byte{}, [32]byte{}, [32]byte{}, nil, err
		}
		values = append(values, key)
	}
	return n, id, authority, values, nil
}
