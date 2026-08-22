package route

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSealedIntroductionSetupIsOpaqueToIntroduction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	client, introduction, service := setupIdentity(t, 71), setupIdentity(t, 72), setupIdentity(t, 73)
	userSocket, serviceSocket := introductionSocketPaths(t)
	manifest, network, epoch := [32]byte{1}, [32]byte{2}, [32]byte{3}
	introductionNode, rendezvousNode := [32]byte{4}, [32]byte{5}
	serviceActor := Actor{Role: "publisher", ManifestDigest: manifest, NetworkID: network, EpochDigest: epoch,
		Certificate: service.certificate, ServiceCertificate: service.certificate,
		IntroductionSetupSocket: serviceSocket, IntroductionSetupPeer: introduction.public,
		IntroductionSetupNode: introductionNode, Deadline: time.Second}
	stopService, serviceDone, err := startIntroductionService(ctx, serviceActor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stopService(); err != nil {
			t.Errorf("clean setup service: %v", err)
		}
	})
	relayActor := Actor{Role: "introduction", Certificate: introduction.certificate,
		IntroductionSetupSocket: userSocket, IntroductionSetupPeer: client.public,
		IntroductionForwardSocket: serviceSocket, IntroductionForwardPublic: service.public, Deadline: time.Second}
	stopRelay, relayDone, err := startIntroductionRelay(ctx, relayActor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stopRelay(); err != nil {
			t.Errorf("clean Introduction relay: %v", err)
		}
	})
	request := Actor{Role: "client", ManifestDigest: manifest, ClientCertificate: client.certificate,
		IntroductionSetupSocket: userSocket, IntroductionSetupPublic: introduction.public,
		IntroductionServicePublic: service.public, Deadline: time.Second,
		Plan: Plan{NetworkID: network, Digest: epoch, ViewRoot: [32]byte{6}, Profile: "h3-route-tracer-v1",
			SelectionAt: time.Now().Unix(),
			Positions: []Position{{Role: "initiator"}, {Role: "introduction", NodeID: introductionNode},
				{Role: "rendezvous", NodeID: rendezvousNode, Endpoint: "198.51.100.9:443"}}}}
	proof, receipt, err := requestIntroductionSetup(ctx, request)
	if err != nil {
		t.Fatal(err)
	}
	serviceResult, relayResult := <-serviceDone, <-relayDone
	if serviceResult.err != nil || relayResult.err != nil {
		t.Fatalf("sealed setup failed: service=%v relay=%v", serviceResult.err, relayResult.err)
	}
	if receipt == [32]byte{} || receipt != serviceResult.receipt || proof != serviceResult.proof {
		t.Fatal("service did not authenticate the exact sealed invitation")
	}
	if relayResult.receipt != [32]byte{} || relayResult.proof != (introductionSetup{}) ||
		relayResult.opaqueBytes == 0 || relayResult.opaqueDigest == [32]byte{} {
		t.Fatal("Introduction did not retain only opaque forwarding evidence")
	}
}

func TestSealedIntroductionSetupRejectsPeerAndBindingMismatch(t *testing.T) {
	for _, test := range []struct {
		name          string
		wrongPeer     bool
		wrongManifest bool
	}{{"peer", true, false}, {"binding", false, true}} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			client, introduction, service := setupIdentity(t, 81), setupIdentity(t, 82), setupIdentity(t, 83)
			attacker := setupIdentity(t, 84)
			userSocket, serviceSocket := introductionSocketPaths(t)
			manifest := [32]byte{11}
			serviceManifest := manifest
			if test.wrongManifest {
				serviceManifest = [32]byte{12}
			}
			stopService, _, err := startIntroductionService(ctx, Actor{Role: "publisher", ManifestDigest: serviceManifest,
				NetworkID: [32]byte{2}, EpochDigest: [32]byte{3}, Certificate: service.certificate,
				ServiceCertificate: service.certificate, IntroductionSetupSocket: serviceSocket,
				IntroductionSetupPeer: introduction.public, IntroductionSetupNode: [32]byte{4}, Deadline: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := stopService(); err != nil {
					t.Errorf("stop sealed setup service: %v", err)
				}
			})
			stopRelay, _, err := startIntroductionRelay(ctx, Actor{Role: "introduction", Certificate: introduction.certificate,
				IntroductionSetupSocket: userSocket, IntroductionSetupPeer: client.public,
				IntroductionForwardSocket: serviceSocket, IntroductionForwardPublic: service.public, Deadline: time.Second})
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := stopRelay(); err != nil {
					t.Errorf("stop sealed setup relay: %v", err)
				}
			})
			certificate := client.certificate
			if test.wrongPeer {
				certificate = attacker.certificate
			}
			_, _, err = requestIntroductionSetup(ctx, Actor{Role: "client", ManifestDigest: manifest,
				ClientCertificate: certificate, IntroductionSetupSocket: userSocket,
				IntroductionSetupPublic: introduction.public, IntroductionServicePublic: service.public,
				Deadline: time.Second, Plan: Plan{NetworkID: [32]byte{2}, Digest: [32]byte{3}, ViewRoot: [32]byte{6},
					Profile: "h3-route-tracer-v1", SelectionAt: time.Now().Unix(), Positions: []Position{{},
						{NodeID: [32]byte{4}}, {NodeID: [32]byte{5}, Endpoint: "198.51.100.10:443"}}}})
			if err == nil {
				t.Fatal("sealed setup accepted a peer or binding mismatch")
			}
		})
	}
}

func TestIntroductionRelayCancellationCleansSocket(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	identity := setupIdentity(t, 91)
	path, _ := introductionSocketPaths(t)
	stop, completed, err := startIntroductionRelay(ctx, Actor{IntroductionSetupSocket: path,
		IntroductionSetupPeer: identity.public, IntroductionForwardSocket: "unused",
		IntroductionForwardPublic: identity.public, Certificate: identity.certificate, Deadline: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-completed:
		if result.err == nil {
			t.Fatal("cancelled Introduction relay completed successfully")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled Introduction relay did not terminate")
	}
	if err := stop(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("Introduction relay socket survived cleanup: %v", err)
	}
}

type sealedSetupIdentity struct {
	public      [32]byte
	certificate tls.Certificate
}

// introductionSocketPaths gives each concurrent test a short, unique AF_UNIX
// directory. The global time-derived names formerly collided under the full
// parallel profile, while testing.T.TempDir can exceed Windows' socket-path
// limit because it embeds the test name.
func introductionSocketPaths(t *testing.T) (string, string) {
	t.Helper()
	directory, err := os.MkdirTemp(os.TempDir(), "asi-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cleanupErr := os.RemoveAll(directory); cleanupErr != nil {
			t.Errorf("clean Introduction socket directory: %v", cleanupErr)
		}
	})
	return filepath.Join(directory, "user.sock"), filepath.Join(directory, "service.sock")
}

func setupIdentity(t *testing.T, marker byte) sealedSetupIdentity {
	t.Helper()
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
	public := private.Public().(ed25519.PublicKey)
	template := &x509.Certificate{SerialNumber: big.NewInt(int64(marker)), Subject: pkix.Name{CommonName: "setup.test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}}
	raw, err := x509.CreateCertificate(nil, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := x509.ParseCertificate(raw)
	if err != nil {
		t.Fatal(err)
	}
	var fixed [32]byte
	copy(fixed[:], public)
	return sealedSetupIdentity{public: fixed,
		certificate: tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: parsed}}
}
