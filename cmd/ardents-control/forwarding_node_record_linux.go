package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"net"
	"strconv"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
)

// The public declaration has one initial Node Record purpose. It contains no
// key path, role assignment, custom message or accepting State projection.
type forwardingRecordPlan struct {
	Schema    string    `json:"schema"`
	Reference time.Time `json:"reference_time"`
	Network   string    `json:"network_id"`
	Node      string    `json:"node_id"`
	PublicKey string    `json:"node_public_key"`
	Family    string    `json:"family"`
	Endpoint  string    `json:"endpoint"`
	From      time.Time `json:"valid_from"`
	Until     time.Time `json:"valid_until"`
	Capacity  uint16    `json:"capacity"`
}

func (p forwardingRecordPlan) current(now time.Time) error {
	if p.Schema != "ardents-forwarding-node-record-plan-v1" || p.Reference.IsZero() || p.Reference.Location() != time.UTC || p.From.Location() != time.UTC || p.Until.Location() != time.UTC || p.Reference.Nanosecond() != 0 || p.Reference.Before(now.Add(-5*time.Minute)) || p.Reference.After(now.Add(time.Minute)) || p.Reference.Before(p.From) || !p.Reference.Before(p.Until) || now.Before(p.From) || !now.Before(p.Until) || p.Until.Sub(p.From) > 7*24*time.Hour {
		return errSourceControlInput
	}
	return nil
}

func forwardingPublic32(raw string) ([32]byte, error) {
	var value [32]byte
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != 32 || hex.EncodeToString(decoded) != raw {
		return value, errSourceControlInput
	}
	copy(value[:], decoded)
	if value == [32]byte{} {
		return value, errSourceControlInput
	}
	return value, nil
}

func (p forwardingRecordPlan) prepare(now time.Time) ([]byte, error) {
	if err := p.current(now); err != nil {
		return nil, err
	}
	host, port, err := net.SplitHostPort(p.Endpoint)
	number, portErr := strconv.ParseUint(port, 10, 16)
	ip := net.ParseIP(host)
	if err != nil || portErr != nil || number < 1024 || ip == nil || net.JoinHostPort(ip.String(), strconv.FormatUint(number, 10)) != p.Endpoint {
		return nil, errSourceControlInput
	}
	network, err := forwardingPublic32(p.Network)
	if err != nil {
		return nil, err
	}
	node, err := forwardingPublic32(p.Node)
	if err != nil {
		return nil, err
	}
	public, err := forwardingPublic32(p.PublicKey)
	if err != nil {
		return nil, err
	}
	return epoch.PrepareInitialClosedRecord(epoch.InitialClosedRecord{NetworkID: network, NodeID: node, PublicKey: ed25519.PublicKey(public[:]), Family: p.Family, Endpoint: p.Endpoint, ValidFrom: p.From, ValidUntil: p.Until, Capacity: p.Capacity, Carrier: epoch.CarrierClosedTCP})
}

func forwardingRecordArguments(args []string, now time.Time) (forwardingRecordPlan, string, error) {
	p := forwardingRecordPlan{Schema: "ardents-forwarding-node-record-plan-v1", Reference: now.UTC().Truncate(time.Second)}
	if len(args) > 20 {
		return p, "", errSourceControlInput
	}
	flags := flag.NewFlagSet("forwarding prepare-node-record", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	var from, until, output string
	var capacity uint
	flags.StringVar(&p.Network, "network-id", "", "explicit Network ID")
	flags.StringVar(&p.Node, "node-id", "", "explicit distinct Node identity")
	flags.StringVar(&p.PublicKey, "node-key", "", "reviewed Node public key")
	flags.StringVar(&p.Family, "family", "", "declared effective operator family")
	flags.StringVar(&p.Endpoint, "endpoint", "", "declared literal TCP endpoint")
	flags.StringVar(&from, "valid-from", "", "whole-second RFC3339 interval start")
	flags.StringVar(&until, "valid-until", "", "whole-second RFC3339 interval end")
	flags.UintVar(&capacity, "capacity", 0, "closed Record capacity, 1..1024")
	flags.StringVar(&output, "output", "", "fresh public preparation directory")
	if flags.Parse(args) != nil || flags.NArg() != 0 || flags.NFlag() != 9 || !forwardingUniqueOptions(args, "") || !sourceControlPath(output) || capacity < 1 || capacity > 1024 {
		return p, "", errSourceControlInput
	}
	p.Capacity = uint16(capacity)
	var err error
	p.From, err = time.Parse(time.RFC3339, from)
	if err != nil || p.From.Format(time.RFC3339) != from {
		return p, "", errSourceControlInput
	}
	p.Until, err = time.Parse(time.RFC3339, until)
	if err != nil || p.Until.Format(time.RFC3339) != until {
		return p, "", errSourceControlInput
	}
	if _, err := p.prepare(now); err != nil {
		return p, "", err
	}
	return p, output, nil
}

func decodeForwardingRecordPlan(raw []byte, now time.Time) (forwardingRecordPlan, []byte, error) {
	var p forwardingRecordPlan
	if len(raw) == 0 || len(raw) > 16<<10 || json.Unmarshal(raw, &p) != nil {
		return p, nil, errSourceControlInput
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return p, nil, errSourceControlInput
	}
	prepared, err := p.prepare(now)
	return p, prepared, err
}
