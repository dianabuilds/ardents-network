package state

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/epoch"
	"github.com/dianabuilds/ardents-network/internal/successor/network/source"
)

func TestSourceResolverRefusesTerminalOwnerFailure(t *testing.T) {
	decision, networkID := sourceResolverGoldenDecision(t)
	request := source.Message{Operation: "latest", NetworkDigest: source.NetworkDigest(networkID)}

	for _, test := range []struct {
		name string
		fail func(*networkState)
	}{
		{name: "automatic refresh", fail: func(s *networkState) { s.automaticErr = errors.New("automatic refresh failed") }},
		{name: "external work refusal", fail: func(s *networkState) { s.config.permitWork = func() error { return errors.New("work unavailable") } }},
	} {
		t.Run(test.name, func(t *testing.T) {
			s := &networkState{config: config{networkID: networkID}, current: &decision}
			before := s.resolveDistributionRequest(context.Background(), request)
			if before.Status != "ok" || before.ObjectDigest != decision.Header.Digest || len(before.Payload) == 0 {
				t.Fatalf("healthy Source response: status=%q digest=%x payload=%d", before.Status, before.ObjectDigest, len(before.Payload))
			}

			test.fail(s)
			if _, err := s.Current(); err == nil {
				t.Fatal("Current did not refuse the terminal owner failure")
			}
			after := s.resolveDistributionRequest(context.Background(), request)
			if after.Status != "busy" || after.ObjectDigest != [32]byte{} || len(after.Payload) != 0 {
				t.Fatalf("Source served after terminal owner failure: status=%q digest=%x payload=%d", after.Status, after.ObjectDigest, len(after.Payload))
			}
		})
	}
}

func sourceResolverGoldenDecision(t *testing.T) (epoch.Decision, [32]byte) {
	t.Helper()
	decode := func(raw []byte) []byte {
		t.Helper()
		value, err := hex.DecodeString(string(bytes.TrimSpace(raw)))
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	read := func(name string) []byte {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		return decode(raw)
	}
	var networkID [32]byte
	copy(networkID[:], decode([]byte("488a631a444652b50d760a739c338d5f7e54bc14e92a3c3d6002eaeead4f2d3d")))
	public := ed25519.PublicKey(decode([]byte("c2f38d34dafe402561da5a0a278e8a3255e0fc9c2e58c0209966a589fd07b631")))
	inputs := make([][]byte, 8)
	for index := range inputs {
		inputs[index] = read(fmt.Sprintf("input-%04d.hex", index))
	}
	decision, err := verifyDecision(config{
		networkID: networkID, authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public},
		threshold: 1, now: time.Unix(1_800_000_100, 0).UTC(),
	}, nil, read("epoch.hex"), inputs, [][]byte{read("materialization-0000.hex")}, true)
	if err != nil {
		t.Fatalf("verify frozen Epoch: %v", err)
	}
	return decision, networkID
}
