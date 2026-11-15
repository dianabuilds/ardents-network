package main

import (
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
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 12 || !forwardingUniqueOptions(args, "authority") || !sourceControlPath(v.destination) || v.wait == 0 || v.wait > 5000 {
		return v, invalid
	}
	return v, nil
}
