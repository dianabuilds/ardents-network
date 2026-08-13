package servicenegative

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"errors"
	"io"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

const attackProofSize = 166

type continuityAttack struct {
	credential serviceconn.Credential
	private    ed25519.PrivateKey
	binding    serviceconn.Recovery
	kind       string

	mu         sync.Mutex
	continuity [32]byte
	replay     []byte
	digest     string
	attempts   uint64
	err        error
}

func (value fixture) observeContinuityAttack(parent context.Context, kind string) RecoveryObservation {
	started := time.Now()
	client, _, publication, ok := value.connected(parent)
	if !ok {
		return RecoveryObservation{}
	}
	binding := serviceconn.Recovery{CandidateView: [32]byte{41}, IsolationContext: [32]byte{42},
		DestinationBinding: [32]byte{43}, RouteProfile: "h3-recovery-negative-v1",
		WorkSafetyNotAfter: value.first.NotAfter, WorkSafetyMaximum: value.first.NotAfter,
		NoNewRecoveryAfter: value.first.NotAfter}
	attack := &continuityAttack{credential: value.first, private: value.firstPrivate, binding: binding, kind: kind}
	clientRoute, maliciousRoute := net.Pipe()
	application, applicationPeer := net.Pipe()
	defer applicationPeer.Close()
	ctx, cancel := context.WithTimeout(parent, 3*time.Second)
	defer cancel()
	go attack.serveInitial(ctx, maliciousRoute)
	outcome := make(chan streamOutcome, 1)
	go func() {
		result, err := client.Do(ctx, serviceconn.Request{Action: "connect", Principal: value.connection,
			Session: sessionFor(client, value, ctx), Target: value.first.Target, Publication: publication,
			Route: clientRoute, Application: application, OpenAttachment: attack.open,
			RecoveryBinding: binding, ReceiveBytes: 1, At: value.now})
		outcome <- streamOutcome{result: result, err: err}
	}()
	select {
	case result := <-outcome:
		attack.mu.Lock()
		digest, injectionErr, attempts := attack.digest, attack.err, attack.attempts
		attack.mu.Unlock()
		passed := result.err != nil && result.result.Class == "abrupt connection loss" &&
			digest != "" && injectionErr == nil
		if kind == "recovery-replayed-attachment" {
			passed = passed && attempts == 2 && result.result.RecoveryCount == 1 && result.result.RouteGeneration == 2
		} else {
			passed = passed && attempts == 1 && result.result.RecoveryCount == 0 && result.result.RouteGeneration == 1
		}
		terminal := uint32(0)
		if passed {
			terminal = 1
		}
		return RecoveryObservation{TerminalCount: terminal, EndpointTerminalCount: terminal,
			Class: result.result.Class, WithinNanos: time.Since(started).Nanoseconds(), Passed: passed,
			InjectionKind: kind, InjectionDigest: digest, AttackAttempts: uint32(attempts),
			RecoveryCount: result.result.RecoveryCount, RouteGeneration: result.result.RouteGeneration}
	case <-ctx.Done():
		return RecoveryObservation{Class: "local timeout or cancellation", WithinNanos: time.Since(started).Nanoseconds()}
	}
}

func (attack *continuityAttack) serveInitial(ctx context.Context, raw net.Conn) {
	connection, continuity, _, err := attack.secure(ctx, raw)
	if err == nil {
		attack.mu.Lock()
		attack.continuity = continuity
		attack.mu.Unlock()
		err = serveInstanceProof(connection, attack.credential, attack.private)
	}
	if connection != nil {
		_ = connection.Close()
	} else {
		_ = raw.Close()
	}
	attack.recordError(err)
}

func (attack *continuityAttack) open(ctx context.Context, request serviceconn.Recovery) (net.Conn, error) {
	endpoint, peer := net.Pipe()
	attack.mu.Lock()
	attack.attempts++
	attempt := attack.attempts
	continuity := attack.continuity
	attack.mu.Unlock()
	if continuity == [32]byte{} || request.Generation < 2 {
		_ = endpoint.Close()
		_ = peer.Close()
		return nil, errors.New("initial continuity is unavailable")
	}
	go attack.serveReplacement(ctx, peer, request.Generation, attempt, continuity)
	return endpoint, nil
}

func (attack *continuityAttack) serveReplacement(ctx context.Context, raw net.Conn,
	generation, attempt uint64, continuity [32]byte) {
	connection, _, exporter, err := attack.secure(ctx, raw)
	if err == nil {
		local := make([]byte, attackProofSize)
		_, err = io.ReadFull(connection, local)
	}
	var proof []byte
	if err == nil {
		proof = attack.proof(continuity, exporter, generation, attempt)
		err = writeAttackAll(connection, proof)
	}
	if connection != nil {
		_ = connection.Close()
	} else {
		_ = raw.Close()
	}
	if proof != nil && !(attack.kind == "recovery-replayed-attachment" && attempt == 1) {
		digest := sha256.Sum256(proof)
		attack.mu.Lock()
		attack.digest = hex.EncodeToString(digest[:])
		attack.mu.Unlock()
	}
	attack.recordError(err)
}

func (attack *continuityAttack) proof(continuity, exporter [32]byte, generation, attempt uint64) []byte {
	attack.mu.Lock()
	defer attack.mu.Unlock()
	if attack.kind == "recovery-replayed-attachment" && attempt > 1 {
		return append([]byte(nil), attack.replay...)
	}
	encodedGeneration := generation
	proofBinding := attack.binding
	if attack.kind == "recovery-stale-attachment" {
		encodedGeneration--
	}
	if attack.kind == "recovery-cross-binding" {
		proofBinding.IsolationContext[0] ^= 1
	}
	proof := encodeAttackProof(continuity, exporter, attack.credential, proofBinding,
		encodedGeneration, generation)
	if attack.kind == "recovery-replayed-attachment" {
		attack.replay = append([]byte(nil), proof...)
	}
	return proof
}

func (attack *continuityAttack) recordError(err error) {
	if err == nil {
		return
	}
	attack.mu.Lock()
	if attack.err == nil {
		attack.err = err
	}
	attack.mu.Unlock()
}

func (attack *continuityAttack) secure(ctx context.Context, raw net.Conn) (*tls.Conn, [32]byte, [32]byte, error) {
	certificate, err := attackCertificate(attack.credential, attack.private)
	if err != nil {
		return nil, [32]byte{}, [32]byte{}, err
	}
	connection := tls.Server(raw, &tls.Config{MinVersion: tls.VersionTLS13, MaxVersion: tls.VersionTLS13,
		Certificates: []tls.Certificate{certificate}, SessionTicketsDisabled: true})
	if err := connection.HandshakeContext(ctx); err != nil {
		return connection, [32]byte{}, [32]byte{}, err
	}
	binding := attackConnectionBinding(attack.credential, attack.binding)
	state := connection.ConnectionState()
	material, err := state.ExportKeyingMaterial(
		"EXPORTER-ardents-h3-service-continuity-v1", binding[:], 32)
	if err != nil {
		return connection, [32]byte{}, [32]byte{}, err
	}
	mac := hmac.New(sha256.New, material)
	_, _ = mac.Write([]byte("ardents-h3-continuity-key-v1\x00"))
	var continuity [32]byte
	copy(continuity[:], mac.Sum(nil))
	exporter := sha256.Sum256(append([]byte("ardents-h3-exporter-commitment-v1\x00"), continuity[:]...))
	return connection, continuity, exporter, nil
}

func attackCertificate(credential serviceconn.Credential, private ed25519.PrivateKey) (tls.Certificate, error) {
	template := &x509.Certificate{SerialNumber: new(big.Int).SetUint64(credential.Generation),
		Subject:   pkix.Name{CommonName: "Ardents qualification attack Instance"},
		NotBefore: time.Unix(credential.NotBefore, 0), NotAfter: time.Unix(credential.NotAfter, 0),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, private.Public(), private)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: private}, nil
}
