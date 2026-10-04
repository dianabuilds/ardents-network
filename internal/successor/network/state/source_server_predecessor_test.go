package state_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/successor/network/duty"
	state2 "github.com/dianabuilds/ardents-network/internal/successor/network/state"
)

func TestServingSourceKeepsPredecessorsUntilAcceptedHandlerJoins(t *testing.T) {
	genesis := newFixture(t)
	second := sourceSingleFieldSuccessor(t, genesis, 2, 0, false)
	third := sourceSingleFieldSuccessor(t, second, 3, 0xa1, true)
	config := installedServingSourceConfig(t, genesis)
	config, client, serverRoot := configureSourceServerWithClient(t, config)
	serving, err := state2.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := serving.Close(); err != nil {
			t.Error(err)
		}
	}()
	firstView, err := serving.Current()
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(serverRoot) {
		t.Fatal("parse serving Source root")
	}
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", config.Source.ServeAddress,
		&tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13, RootCAs: roots,
			ServerName: "source-duty-server.test", Certificates: []tls.Certificate{client.certificate},
			Time: func() time.Time { return time.Unix(genesis.now, 0).UTC() }})
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	secondView, err := serving.Accept(context.Background(), second.epoch, second.inputs, second.materializations)
	if err != nil {
		t.Fatal(err)
	}
	thirdView, err := serving.Accept(context.Background(), third.epoch, third.inputs, third.materializations)
	if err != nil {
		t.Fatal(err)
	}
	if firstView.SourceNodeID != secondView.SourceNodeID || firstView.SourceFamily == secondView.SourceFamily ||
		secondView.SourceNodeID == thirdView.SourceNodeID || secondView.SourceFamily != thirdView.SourceFamily {
		t.Fatalf("expected family-only then identity-only Source transitions: A=%x/%q B=%x/%q C=%x/%q",
			firstView.SourceNodeID, firstView.SourceFamily, secondView.SourceNodeID, secondView.SourceFamily,
			thirdView.SourceNodeID, thirdView.SourceFamily)
	}
	views := []state2.Snapshot{firstView, secondView, thirdView}
	for _, view := range views {
		family := sha256.Sum256([]byte(view.SourceFamily))
		protected, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return time.Unix(genesis.now, 0).UTC() }, view.SourceNodeID, family)
		if err != nil || !protected {
			t.Fatalf("Source Epoch %d predecessor lost while handler active: protected=%t err=%v", view.Epoch, protected, err)
		}
	}
	firstFamily := sha256.Sum256([]byte(firstView.SourceFamily))
	secondFamily := sha256.Sum256([]byte(secondView.SourceFamily))
	if held, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return time.Unix(genesis.now, 0).UTC() },
		[32]byte{}, firstFamily); err != nil || !held {
		t.Fatalf("family-only predecessor guard A: held=%t err=%v", held, err)
	}
	if held, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return time.Unix(genesis.now, 0).UTC() },
		secondView.SourceNodeID, [32]byte{}); err != nil || !held {
		t.Fatalf("identity-only predecessor guard B: held=%t err=%v", held, err)
	}
	if err := connection.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		firstHeld, firstErr := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return time.Unix(genesis.now, 0).UTC() }, [32]byte{}, firstFamily)
		secondHeld, secondErr := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return time.Unix(genesis.now, 0).UTC() }, secondView.SourceNodeID, [32]byte{})
		if firstErr != nil || secondErr != nil {
			t.Fatalf("read joined predecessor guards: first=%v second=%v", firstErr, secondErr)
		}
		if !firstHeld && !secondHeld {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("joined handler left predecessor guards: first=%t second=%t", firstHeld, secondHeld)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if held, err := duty.ReadConflict(config.LocalRoleStateRoot, func() time.Time { return time.Unix(genesis.now, 0).UTC() },
		thirdView.SourceNodeID, secondFamily); err != nil || !held {
		t.Fatalf("current Source guard C after handler join: held=%t err=%v", held, err)
	}
}

func sourceSingleFieldSuccessor(t *testing.T, previous fixture, number uint64, firstMarker byte, changeIdentity bool) fixture {
	t.Helper()
	next := previous
	next.inputs = append([][]byte(nil), previous.inputs...)
	next.accepted = append([]fixtureRecord(nil), previous.accepted...)
	for index, prior := range previous.accepted {
		marker := firstMarker + byte(index)
		var family string
		if changeIdentity {
			// Keep the selected predecessor family while rotating its identity.
			// This fixture's assignment selects the opposite candidate at C.
			family = previous.accepted[1-index].family
		} else {
			family = fmt.Sprintf("source-successor-%d-%d", number, index)
			switch prior.nodeID {
			case sha256.Sum256([]byte{0x4e, 0x11}):
				marker = 0x11
			case sha256.Sum256([]byte{0x4e, 0x21}):
				marker = 0x21
			default:
				t.Fatalf("unexpected original Source identity %x", prior.nodeID)
			}
		}
		newRecord := makeRecord(t, previous.networkID, marker, family,
			fmt.Sprintf("127.0.0.1:%d", 4400+number*10+uint64(index)), prior.capacity)
		for inputIndex, input := range next.inputs {
			if bytes.Equal(input, prior.bytes) {
				next.inputs[inputIndex] = newRecord.bytes
			}
		}
		next.accepted[index] = newRecord
	}
	now := time.Unix(previous.now, 0).UTC()
	return buildFixtureEpoch(t, next, number, previous.epochDigest,
		sha256.Sum256([]byte("assignment-seed-1")),
		now.Add(-30*time.Second), now.Add(time.Duration(number)*time.Hour))
}
