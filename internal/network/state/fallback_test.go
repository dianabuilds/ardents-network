package state_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

func TestIncompleteLatestUsesExactSameIndexFallback(t *testing.T) {
	genesis := newFixture(t)
	successor := nextFixture(t, genesis)
	now := time.Unix(genesis.now, 0).UTC()
	clientAuthority := makeTestAuthority(t, 0x81, "fallback-client-root")
	client := makeTestLeaf(t, clientAuthority, 0x82, "fallback-endpoint.test", false)
	firstAuthority := makeTestAuthority(t, 0x83, "truncated-source-root")
	secondAuthority := makeTestAuthority(t, 0x84, "complete-source-root")
	firstServer := makeTestLeaf(t, firstAuthority, 0x85, "truncated-source.test", true)
	secondServer := makeTestLeaf(t, secondAuthority, 0x86, "complete-source.test", true)
	addresses := [2]string{availableAddress(t), availableAddress(t)}
	stopFirst := startTruncatedSource(t, addresses[0], firstServer, clientAuthority.rootPEM, successor.epochDigest)
	defer stopFirst()
	second := openTestSourceAtIndex(t, successor, addresses[1], secondServer, clientAuthority.rootPEM, client.pin, 1)
	defer second.Close()

	config := fixtureConfig(genesis, t.TempDir(), now)
	installed, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := installed.Accept(context.Background(), genesis.epoch, genesis.inputs, genesis.materializations); err != nil {
		t.Fatal(err)
	}
	if err := installed.Close(); err != nil {
		t.Fatal(err)
	}
	config.Now, config.Clock = time.Time{}, func() time.Time { return now }
	config.ClockObservation = now
	config.Source.Addresses = addresses
	config.Source.ServerNames = [2]string{"truncated-source.test", "complete-source.test"}
	config.Source.Identities = [2][32]byte{{1}, {2}}
	config.Source.Families = [2]string{"truncated-family", "complete-family"}
	config.Source.EndpointHandles = [2]string{"truncated-handle", "complete-handle"}
	config.Source.RootPEM = [2][]byte{firstAuthority.rootPEM, secondAuthority.rootPEM}
	config.Source.LeafKeyDigests = [2][32]byte{firstServer.pin, secondServer.pin}
	config.Source.ClientCertificate = client.certificate
	config.Source.MaterialIndex = 1
	endpoint, err := state.Open(config)
	if err != nil {
		t.Fatal(err)
	}
	defer endpoint.Close()
	refreshed, err := endpoint.Refresh(context.Background())
	if err != nil {
		t.Fatalf("same-index BY_DIGEST fallback: %v", err)
	}
	wantOutcomes := [4]string{"framing-failed", "valid", "not-attempted", "valid"}
	if refreshed.Epoch != 2 || refreshed.SourceOutcomes != wantOutcomes ||
		refreshed.ObservedEpochs[1] != 2 || refreshed.ObservedEpochs[3] != 2 {
		t.Fatalf("fallback evidence=%+v", refreshed)
	}
}

func startTruncatedSource(t *testing.T, address string, server testCertificate, clientRoot []byte, digest [32]byte) func() {
	t.Helper()
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(clientRoot) {
		t.Fatal("append fallback client root")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	tlsListener := tls.NewListener(listener, &tls.Config{
		MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{server.certificate}, ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs: roots, SessionTicketsDisabled: true,
	})
	var active sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			connection, acceptErr := tlsListener.Accept()
			if acceptErr != nil {
				return
			}
			active.Add(1)
			go func() {
				defer active.Done()
				defer connection.Close()
				_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
				var request [77]byte
				if _, err := io.ReadFull(connection, request[:]); err != nil {
					return
				}
				var response [46]byte
				copy(response[:8], "ARDH3S1\x00")
				copy(response[9:41], digest[:])
				binary.BigEndian.PutUint32(response[41:45], 16)
				_, _ = connection.Write(response[:])
			}()
		}
	}()
	return func() {
		_ = tlsListener.Close()
		<-done
		active.Wait()
	}
}
