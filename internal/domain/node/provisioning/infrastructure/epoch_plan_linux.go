package infrastructure

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"flag"
	"github.com/dianabuilds/ardents-network/internal/platform/artifacts"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dianabuilds/ardents-network/internal/domain/network/epoch"
)

const maximumNodeEpochPlan = 8 << 20

type nodeEpochRecord struct {
	Declaration nodeRecordPlan `json:"declaration"`
	Signed      []byte         `json:"signed"`
}

type nodeEpochPlan struct {
	Schema    string            `json:"schema"`
	Reference time.Time         `json:"reference_time"`
	Network   string            `json:"network_id"`
	Authority string            `json:"state_authority_key"`
	Seed      string            `json:"assignment_seed"`
	Node      string            `json:"materialized_node_id"`
	From      time.Time         `json:"valid_from"`
	Until     time.Time         `json:"valid_until"`
	Domains   []string          `json:"domains"`
	Records   []nodeEpochRecord `json:"records"`
}

func (p nodeEpochPlan) current(role nodeProvisioningRole, now time.Time) error {
	if !role.valid() || p.Schema != role.schema("epoch-plan") || p.Reference.IsZero() || p.Reference.Location() != time.UTC || p.From.Location() != time.UTC || p.Until.Location() != time.UTC || p.Reference.Nanosecond() != 0 || p.Reference.Before(now.Add(-5*time.Minute)) || p.Reference.After(now.Add(time.Minute)) || p.Reference.Before(p.From) || !p.Reference.Before(p.Until) || now.Before(p.From) || !now.Before(p.Until) || p.Until.Sub(p.From) > 7*24*time.Hour {
		return artifacts.ErrInput
	}
	return nil
}

func (p nodeEpochPlan) prepare(role nodeProvisioningRole, now time.Time) ([]byte, error) {
	if err := p.current(role, now); err != nil {
		return nil, err
	}
	network, err := controlPublic32(p.Network)
	if err != nil {
		return nil, err
	}
	authority, err := controlPublic32(p.Authority)
	if err != nil {
		return nil, err
	}
	seed, err := controlPublic32(p.Seed)
	if err != nil {
		return nil, err
	}
	if _, err := controlPublic32(p.Node); err != nil {
		return nil, err
	}
	if len(p.Records) == 0 || len(p.Records) > 64 || len(p.Domains) == 0 || len(p.Domains) > 16 {
		return nil, artifacts.ErrInput
	}
	records := make([][]byte, 0, len(p.Records))
	selected := false
	for _, record := range p.Records {
		// This old public reference validates its original declaration only.
		// Signed Record validity/current eligibility belongs to the new Epoch
		// operation below; this does not renew a past signing permission.
		message, err := record.Declaration.prepare(role, record.Declaration.Reference)
		if err != nil || record.Declaration.Network != p.Network || record.Declaration.PublicKey == p.Authority || len(record.Signed) > epoch.MaxRecordBytes {
			return nil, artifacts.ErrInput
		}
		if _, err := epoch.VerifyInitialClosedRecord(message, record.Signed); err != nil {
			return nil, err
		}
		selected = selected || record.Declaration.Node == p.Node
		records = append(records, record.Signed)
	}
	if !selected {
		return nil, artifacts.ErrInput
	}
	return epoch.PrepareInitialClosedEpoch(epoch.InitialClosedEpoch{NetworkID: network, AssignmentSeed: seed, ValidFrom: p.From, ValidUntil: p.Until, Domains: p.Domains, Records: records, AuthorityKeys: []ed25519.PublicKey{ed25519.PublicKey(authority[:])}, Carrier: epoch.CarrierClosedTCP})
}

func decodeNodeEpochPlan(role nodeProvisioningRole, raw []byte, now time.Time) (nodeEpochPlan, []byte, error) {
	var p nodeEpochPlan
	if len(raw) == 0 || len(raw) > maximumNodeEpochPlan || json.Unmarshal(raw, &p) != nil {
		return p, nil, artifacts.ErrInput
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return p, nil, artifacts.ErrInput
	}
	prepared, err := p.prepare(role, now)
	return p, prepared, err
}

type nodeEpochFlags struct {
	values  []string
	maximum int
}

func (f *nodeEpochFlags) String() string { return strings.Join(f.values, ",") }
func (f *nodeEpochFlags) Set(value string) error {
	if len(f.values) >= f.maximum {
		return artifacts.ErrInput
	}
	f.values = append(f.values, value)
	return nil
}

func prepareNodeEpoch(ctx context.Context, role nodeProvisioningRole, args []string, observe func(string, *os.File)) (p nodeEpochPlan, result []byte, returned error) {
	if !role.valid() || ctx == nil || ctx.Err() != nil || os.Geteuid() != 0 || len(args) > 320 {
		return p, nil, artifacts.ErrInput
	}
	p = nodeEpochPlan{Schema: role.schema("epoch-plan"), Reference: time.Now().UTC().Truncate(time.Second)}
	flags := flag.NewFlagSet(string(role)+" prepare-epoch", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var from, until, output string
	domains, plans, records := nodeEpochFlags{maximum: 16}, nodeEpochFlags{maximum: 64}, nodeEpochFlags{maximum: 64}
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
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 10 || !artifacts.UniqueOptions(args, "domain", "record-plan", "record") || !artifacts.AbsolutePath(output) || len(plans.values) == 0 || len(plans.values) != len(records.values) {
		return p, nil, artifacts.ErrInput
	}
	var err error
	p.From, err = time.Parse(time.RFC3339, from)
	if err != nil || p.From.Format(time.RFC3339) != from {
		return p, nil, artifacts.ErrInput
	}
	p.Until, err = time.Parse(time.RFC3339, until)
	if err != nil || p.Until.Format(time.RFC3339) != until {
		return p, nil, artifacts.ErrInput
	}
	p.Domains = append([]string(nil), domains.values...)
	inputs := artifacts.NewInputs(ctx)
	defer func() {
		returned = errors.Join(returned, inputs.Close(), ctx.Err())
		if p.Schema != "" {
			returned = errors.Join(returned, p.current(role, time.Now().UTC()))
		}
		if returned != nil {
			result = nil
		}
	}()
	seen := map[string]bool{}
	for i, path := range plans.values {
		for _, input := range []string{path, records.values[i]} {
			if !artifacts.AbsolutePath(input) || seen[input] || input == output || strings.HasPrefix(input, output+"/") {
				return p, nil, artifacts.ErrInput
			}
			seen[input] = true
		}
		raw, err := inputs.Read(path, 16<<10)
		if err != nil {
			return p, nil, err
		}
		var declaration nodeRecordPlan
		if json.Unmarshal(raw, &declaration) != nil {
			return p, nil, artifacts.ErrInput
		}
		declaration, _, err = decodeNodeRecordPlan(role, raw, declaration.Reference)
		if err != nil {
			return p, nil, err
		}
		signed, err := inputs.Read(records.values[i], epoch.MaxRecordBytes)
		if err != nil {
			return p, nil, err
		}
		p.Records = append(p.Records, nodeEpochRecord{declaration, bytes.Clone(signed)})
	}
	result, err = p.prepare(role, time.Now().UTC())
	if err != nil {
		return p, nil, err
	}
	raw, err := json.Marshal(p)
	if err != nil || len(raw)+1 > maximumNodeEpochPlan {
		return p, nil, artifacts.ErrInput
	}
	check := func() error { return errors.Join(p.current(role, time.Now().UTC()), inputs.CheckProtected()) }
	if err := check(); err != nil {
		return p, nil, err
	}
	if err := artifacts.Publish(ctx, output, []artifacts.Content{{Name: "epoch-plan.json", Data: append(raw, '\n')}, {Name: "epoch.unsigned", Data: result}}, check, observe); err != nil {
		return p, nil, err
	}
	return p, bytes.Clone(result), check()
}
