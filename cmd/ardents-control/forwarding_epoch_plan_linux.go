package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

const maximumForwardingEpochPlan = 8 << 20

type forwardingEpochRecord struct {
	Declaration forwardingRecordPlan `json:"declaration"`
	Signed      []byte               `json:"signed"`
}

type forwardingEpochPlan struct {
	Schema    string                  `json:"schema"`
	Reference time.Time               `json:"reference_time"`
	Network   string                  `json:"network_id"`
	Authority string                  `json:"state_authority_key"`
	Seed      string                  `json:"assignment_seed"`
	Node      string                  `json:"materialized_node_id"`
	From      time.Time               `json:"valid_from"`
	Until     time.Time               `json:"valid_until"`
	Domains   []string                `json:"domains"`
	Records   []forwardingEpochRecord `json:"records"`
}

func (p forwardingEpochPlan) current(now time.Time) error {
	if p.Schema != "ardents-forwarding-epoch-plan-v1" || p.Reference.IsZero() || p.Reference.Location() != time.UTC || p.From.Location() != time.UTC || p.Until.Location() != time.UTC || p.Reference.Nanosecond() != 0 || p.Reference.Before(now.Add(-5*time.Minute)) || p.Reference.After(now.Add(time.Minute)) || p.Reference.Before(p.From) || !p.Reference.Before(p.Until) || now.Before(p.From) || !now.Before(p.Until) || p.Until.Sub(p.From) > 7*24*time.Hour {
		return errSourceControlInput
	}
	return nil
}

func (p forwardingEpochPlan) prepare(now time.Time) ([]byte, error) {
	if err := p.current(now); err != nil {
		return nil, err
	}
	network, err := forwardingPublic32(p.Network)
	if err != nil {
		return nil, err
	}
	authority, err := forwardingPublic32(p.Authority)
	if err != nil {
		return nil, err
	}
	seed, err := forwardingPublic32(p.Seed)
	if err != nil {
		return nil, err
	}
	if _, err := forwardingPublic32(p.Node); err != nil {
		return nil, err
	}
	if len(p.Records) == 0 || len(p.Records) > 64 || len(p.Domains) == 0 || len(p.Domains) > 16 {
		return nil, errSourceControlInput
	}
	records := make([][]byte, 0, len(p.Records))
	selected := false
	for _, record := range p.Records {
		// This old public reference validates its original declaration only.
		// Signed Record validity/current eligibility belongs to the new Epoch
		// operation below; this does not renew a past signing permission.
		message, err := record.Declaration.prepare(record.Declaration.Reference)
		if err != nil || record.Declaration.Network != p.Network || record.Declaration.PublicKey == p.Authority || len(record.Signed) > epoch.MaxRecordBytes {
			return nil, errSourceControlInput
		}
		if _, err := epoch.VerifyInitialClosedRecord(message, record.Signed); err != nil {
			return nil, err
		}
		selected = selected || record.Declaration.Node == p.Node
		records = append(records, record.Signed)
	}
	if !selected {
		return nil, errSourceControlInput
	}
	return epoch.PrepareInitialClosedEpoch(epoch.InitialClosedEpoch{NetworkID: network, AssignmentSeed: seed, ValidFrom: p.From, ValidUntil: p.Until, Domains: p.Domains, Records: records, AuthorityKeys: []ed25519.PublicKey{ed25519.PublicKey(authority[:])}, Carrier: epoch.CarrierClosedTCP})
}

func decodeForwardingEpochPlan(raw []byte, now time.Time) (forwardingEpochPlan, []byte, error) {
	var p forwardingEpochPlan
	if len(raw) == 0 || len(raw) > maximumForwardingEpochPlan || json.Unmarshal(raw, &p) != nil {
		return p, nil, errSourceControlInput
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return p, nil, errSourceControlInput
	}
	prepared, err := p.prepare(now)
	return p, prepared, err
}

type forwardingEpochFlags struct {
	values  []string
	maximum int
}

func (f *forwardingEpochFlags) String() string { return strings.Join(f.values, ",") }
func (f *forwardingEpochFlags) Set(value string) error {
	if len(f.values) >= f.maximum {
		return errSourceControlInput
	}
	f.values = append(f.values, value)
	return nil
}

func prepareForwardingEpoch(ctx context.Context, args []string, observe func(string, *os.File)) (p forwardingEpochPlan, result []byte, returned error) {
	if ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) > 320 {
		return p, nil, errSourceControlInput
	}
	p = forwardingEpochPlan{Schema: "ardents-forwarding-epoch-plan-v1", Reference: time.Now().UTC().Truncate(time.Second)}
	flags := flag.NewFlagSet("forwarding prepare-epoch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var from, until, output string
	domains, plans, records := forwardingEpochFlags{maximum: 16}, forwardingEpochFlags{maximum: 64}, forwardingEpochFlags{maximum: 64}
	flags.StringVar(&p.Network, "network-id", "", "explicit Network ID")
	flags.StringVar(&p.Authority, "authority-key", "", "independently reviewed State authority public key")
	flags.StringVar(&p.Seed, "assignment-seed", "", "fixed reviewed assignment seed; no redraw")
	flags.StringVar(&p.Node, "node-id", "", "exact Node to materialize, not a role selector")
	flags.StringVar(&from, "valid-from", "", "whole-second UTC Epoch start")
	flags.StringVar(&until, "valid-until", "", "whole-second UTC Epoch end")
	flags.Var(&domains, "domain", "canonical ordered Role Domain, repeat at most16")
	flags.Var(&plans, "record-plan", "original public Node declaration, paired by order")
	flags.Var(&records, "record", "original signed Node Record, paired by order")
	flags.StringVar(&output, "output", "", "fresh public Epoch preparation output")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 10 || !forwardingEpochUniqueOptions(args) || !sourceControlPath(output) || len(plans.values) == 0 || len(plans.values) != len(records.values) {
		return p, nil, errSourceControlInput
	}
	var err error
	p.From, err = time.Parse(time.RFC3339, from)
	if err != nil || p.From.Format(time.RFC3339) != from {
		return p, nil, errSourceControlInput
	}
	p.Until, err = time.Parse(time.RFC3339, until)
	if err != nil || p.Until.Format(time.RFC3339) != until {
		return p, nil, errSourceControlInput
	}
	p.Domains = append([]string(nil), domains.values...)
	inputs := &sourceControlInputs{ctx: ctx}
	defer func() {
		returned = errors.Join(returned, inputs.close(), ctx.Err())
		if p.Schema != "" {
			returned = errors.Join(returned, p.current(time.Now().UTC()))
		}
		if returned != nil {
			result = nil
		}
	}()
	seen := map[string]bool{}
	for i, path := range plans.values {
		for _, input := range []string{path, records.values[i]} {
			if !sourceControlPath(input) || seen[input] || input == output || strings.HasPrefix(input, output+"/") {
				return p, nil, errSourceControlInput
			}
			seen[input] = true
		}
		raw, err := inputs.read(path, 16<<10)
		if err != nil {
			return p, nil, err
		}
		var declaration forwardingRecordPlan
		if json.Unmarshal(raw, &declaration) != nil {
			return p, nil, errSourceControlInput
		}
		declaration, _, err = decodeForwardingRecordPlan(raw, declaration.Reference)
		if err != nil {
			return p, nil, err
		}
		signed, err := inputs.read(records.values[i], epoch.MaxRecordBytes)
		if err != nil {
			return p, nil, err
		}
		p.Records = append(p.Records, forwardingEpochRecord{declaration, bytes.Clone(signed)})
	}
	result, err = p.prepare(time.Now().UTC())
	if err != nil {
		return p, nil, err
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw)+1 > maximumForwardingEpochPlan {
		return p, nil, errSourceControlInput
	}
	check := func() error { return errors.Join(p.current(time.Now().UTC()), inputs.checkProtected()) }
	if err := check(); err != nil {
		return p, nil, err
	}
	if err := persistSourceArtifacts(ctx, output, []sourceArtifactFile{{"epoch-plan.json", append(raw, '\n')}, {"epoch.unsigned", result}}, check, observe); err != nil {
		return p, nil, err
	}
	return p, bytes.Clone(result), check()
}

func forwardingEpochUniqueOptions(args []string) bool {
	seen := map[string]bool{}
	for _, argument := range args {
		if len(argument) < 2 || argument[0] != '-' {
			continue
		}
		name := strings.TrimLeft(argument, "-")
		name, _, _ = strings.Cut(name, "=")
		if seen[name] && name != "domain" && name != "record-plan" && name != "record" {
			return false
		}
		seen[name] = true
	}
	return true
}
