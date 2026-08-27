//go:build ignore

package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

type environmentResult struct {
	Schema string `json:"schema"`
	Go     string `json:"go"`
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	QUICGo string `json:"quic_go"`
}

type caseResult struct {
	Schema       string      `json:"schema"`
	Case         string      `json:"case"`
	Profile      profileID   `json:"profile,omitempty"`
	Expected     resultClass `json:"expected"`
	Observed     resultClass `json:"observed"`
	Passed       bool        `json:"passed"`
	AdapterCalls string      `json:"adapter_calls"`
	OfferDigest  string      `json:"offer_digest,omitempty"`
	Bytes        int         `json:"bytes,omitempty"`
	Cleanup      bool        `json:"cleanup_joined"`
	ElapsedMS    int64       `json:"elapsed_ms"`
	Note         string      `json:"note,omitempty"`
}

func main() {
	material, err := newIdentities()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(environment())
	results := []caseResult{}
	for _, profile := range []profileID{tcpProfile, quicProfile} {
		results = append(results,
			runHappy(material, profile),
			runWrongPeer(material, profile),
			runWrongBinding(material, profile),
			runHandshakeStall(material, profile),
			runBackpressure(material, profile),
			runAttachmentLoss(material, profile),
		)
	}
	results = append(results, runExpiredOffer(material), runUnknownProfile(material), runRecoveryShape(material))
	failed := false
	for _, result := range results {
		_ = encoder.Encode(result)
		failed = failed || !result.Passed
	}
	if failed {
		os.Exit(1)
	}
}

func environment() environmentResult {
	version := "unavailable"
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dependency := range info.Deps {
			if dependency.Path == "github.com/quic-go/quic-go" {
				version = dependency.Version
			}
		}
	}
	return environmentResult{Schema: "ardents-r094-environment-v1", Go: runtime.Version(), OS: runtime.GOOS,
		Arch: runtime.GOARCH, QUICGo: version}
}

func runHappy(material identities, profile profileID) caseResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deadline := futureDeadline(4 * time.Second)
	attempt := newAttempt(material, profile, "happy-"+string(profile), deadline)
	peer, err := startPeer(ctx, profile, material.server, material.clientID, reciprocal(attempt.Binding), peerNormal)
	if err != nil {
		return failedSetup("happy", profile, classSuccess, started, err)
	}
	attempt.Endpoint = peer.Endpoint
	module := newCarrierModule()
	carrier, openErr := module.Open(ctx, attempt)
	transcriptErr := error(nil)
	closeErr := error(nil)
	if openErr == nil {
		transcriptErr = exchange(carrier)
		closeErr = carrier.Close()
	}
	peerErr := peer.Close()
	observed := classOf(errors.Join(openErr, transcriptErr, closeErr, peerErr))
	passed := observed == classSuccess && selectedOnce(module, profile)
	return result("happy", profile, classSuccess, observed, passed, module, attempt, len(requestBytes)+len(responseBytes),
		true, started, joinNote(openErr, transcriptErr, closeErr, peerErr))
}

func runWrongPeer(material identities, profile profileID) caseResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deadline := futureDeadline(4 * time.Second)
	attempt := newAttempt(material, profile, "wrong-peer-"+string(profile), deadline)
	peer, err := startPeer(ctx, profile, material.wrongServer, material.clientID, reciprocal(attempt.Binding), peerNormal)
	if err != nil {
		return failedSetup("wrong-peer", profile, classUnauthorized, started, err)
	}
	attempt.Endpoint = peer.Endpoint
	module := newCarrierModule()
	carrier, openErr := module.Open(ctx, attempt)
	if carrier != nil {
		_ = carrier.Close()
	}
	_ = peer.Close()
	observed := classOf(openErr)
	return result("wrong-peer", profile, classUnauthorized, observed,
		observed == classUnauthorized && selectedOnce(module, profile), module, attempt, 0, true, started, joinNote(openErr))
}

func runWrongBinding(material identities, profile profileID) caseResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deadline := futureDeadline(4 * time.Second)
	attempt := newAttempt(material, profile, "wrong-binding-"+string(profile), deadline)
	peerBinding := reciprocal(attempt.Binding)
	peerBinding.AttachmentID = sha256.Sum256([]byte("r094-wrong-attachment"))
	peer, err := startPeer(ctx, profile, material.server, material.clientID, peerBinding, peerNormal)
	if err != nil {
		return failedSetup("wrong-binding", profile, classUnauthorized, started, err)
	}
	attempt.Endpoint = peer.Endpoint
	module := newCarrierModule()
	carrier, openErr := module.Open(ctx, attempt)
	if carrier != nil {
		_ = carrier.Close()
	}
	_ = peer.Close()
	observed := classOf(openErr)
	return result("wrong-binding", profile, classUnauthorized, observed,
		observed == classUnauthorized && selectedOnce(module, profile), module, attempt, 0, true, started, joinNote(openErr))
}

func runHandshakeStall(material identities, profile profileID) caseResult {
	started := time.Now()
	peerCtx, stopPeer := context.WithCancel(context.Background())
	defer stopPeer()
	peer, err := startStall(peerCtx, profile)
	if err != nil {
		return failedSetup("handshake-stall", profile, classTimeout, started, err)
	}
	attempt := newAttempt(material, profile, "handshake-stall-"+string(profile), futureDeadline(5*time.Second))
	attempt.Endpoint = peer.Endpoint
	module := newCarrierModule()
	ctx, cancel := context.WithTimeout(context.Background(), 350*time.Millisecond)
	carrier, openErr := module.Open(ctx, attempt)
	cancel()
	if carrier != nil {
		_ = carrier.Close()
	}
	_ = peer.Close()
	observed := classOf(openErr)
	return result("handshake-stall", profile, classTimeout, observed,
		observed == classTimeout && selectedOnce(module, profile), module, attempt, 0, true, started, joinNote(openErr))
}

func runBackpressure(material identities, profile profileID) caseResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	deadline := futureDeadline(2 * time.Second)
	attempt := newAttempt(material, profile, "backpressure-"+string(profile), deadline)
	peer, err := startPeer(ctx, profile, material.server, material.clientID, reciprocal(attempt.Binding), peerStallData)
	if err != nil {
		return failedSetup("backpressure", profile, classTimeout, started, err)
	}
	attempt.Endpoint = peer.Endpoint
	module := newCarrierModule()
	carrier, openErr := module.Open(ctx, attempt)
	written := 0
	writeErr := error(nil)
	if openErr == nil {
		payload := make([]byte, 32<<10)
		for written < 16<<20 && writeErr == nil {
			var count int
			count, writeErr = carrier.Write(payload)
			written += count
		}
		_ = carrier.Close()
	}
	_ = peer.Close()
	observed := classOf(errors.Join(openErr, writeErr))
	passed := observed == classTimeout && written < 16<<20 && selectedOnce(module, profile)
	return result("backpressure", profile, classTimeout, observed, passed, module, attempt, written, true, started,
		joinNote(openErr, writeErr))
}

func runAttachmentLoss(material identities, profile profileID) caseResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	attempt := newAttempt(material, profile, "loss-"+string(profile), futureDeadline(4*time.Second))
	peer, err := startPeer(ctx, profile, material.server, material.clientID, reciprocal(attempt.Binding), peerLossAfterBinding)
	if err != nil {
		return failedSetup("attachment-loss", profile, classUnavailable, started, err)
	}
	attempt.Endpoint = peer.Endpoint
	module := newCarrierModule()
	carrier, openErr := module.Open(ctx, attempt)
	readErr := error(nil)
	if openErr == nil {
		_, _ = carrier.Write([]byte(lossTrigger))
		buffer := make([]byte, 1)
		_, readErr = carrier.Read(buffer)
		_ = carrier.Close()
	}
	_ = peer.Close()
	observed := classOf(errors.Join(openErr, readErr))
	return result("attachment-loss", profile, classUnavailable, observed,
		observed == classUnavailable && selectedOnce(module, profile), module, attempt, 0, true, started, joinNote(openErr, readErr))
}

func runExpiredOffer(material identities) caseResult {
	started := time.Now()
	deadline := time.Now().Add(-time.Second).UTC().Truncate(time.Second)
	attempt := newAttempt(material, tcpProfile, "expired", deadline)
	attempt.Endpoint = "127.0.0.1:1"
	module := newCarrierModule()
	_, err := module.Open(context.Background(), attempt)
	observed := classOf(err)
	return result("expired-offer", tcpProfile, classStale, observed, observed == classStale && totalCalls(module) == 0,
		module, attempt, 0, true, started, joinNote(err))
}

func runUnknownProfile(material identities) caseResult {
	started := time.Now()
	attempt := newAttempt(material, profileID("r094-unknown"), "unknown", futureDeadline(4*time.Second))
	attempt.Endpoint = "127.0.0.1:1"
	module := newCarrierModule()
	_, err := module.Open(context.Background(), attempt)
	observed := classOf(err)
	return result("unknown-profile", attempt.Profile, classIncompatible, observed,
		observed == classIncompatible && totalCalls(module) == 0, module, attempt, 0, true, started, joinNote(err))
}

func runRecoveryShape(material identities) caseResult {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	module := newCarrierModule()
	first := newAttempt(material, tcpProfile, "recovery-first", futureDeadline(4*time.Second))
	firstPeer, err := startPeer(ctx, tcpProfile, material.server, material.clientID, reciprocal(first.Binding), peerLossAfterBinding)
	if err != nil {
		return failedSetup("new-authorized-attachment", quicProfile, classSuccess, started, err)
	}
	first.Endpoint = firstPeer.Endpoint
	firstCarrier, firstOpen := module.Open(ctx, first)
	firstLoss := error(nil)
	if firstOpen == nil {
		_, _ = firstCarrier.Write([]byte(lossTrigger))
		_, firstLoss = firstCarrier.Read(make([]byte, 1))
		_ = firstCarrier.Close()
	}
	_ = firstPeer.Close()

	second := newAttempt(material, quicProfile, "recovery-second", futureDeadline(4*time.Second))
	secondPeer, secondStart := startPeer(ctx, quicProfile, material.server, material.clientID, reciprocal(second.Binding), peerNormal)
	if secondStart != nil {
		return failedSetup("new-authorized-attachment", quicProfile, classSuccess, started, secondStart)
	}
	second.Endpoint = secondPeer.Endpoint
	secondCarrier, secondOpen := module.Open(ctx, second)
	secondExchange := error(nil)
	if secondOpen == nil {
		secondExchange = exchange(secondCarrier)
		_ = secondCarrier.Close()
	}
	secondPeerErr := secondPeer.Close()
	observed := classOf(errors.Join(secondOpen, secondExchange, secondPeerErr))
	passed := classOf(errors.Join(firstOpen, firstLoss)) == classUnavailable && observed == classSuccess &&
		first.Binding.AttachmentID != second.Binding.AttachmentID && module.calls[tcpProfile] == 1 && module.calls[quicProfile] == 1
	return result("new-authorized-attachment", quicProfile, classSuccess, observed, passed, module, second,
		len(requestBytes)+len(responseBytes), true, started,
		"first TCP attachment lost; second State-shaped QUIC attempt used a distinct attachment id")
}

func startPeer(ctx context.Context, profile profileID, certificate tls.Certificate, expectedClient [32]byte,
	binding route.LegBinding, mode peerMode) (*peerRuntime, error) {
	switch profile {
	case tcpProfile:
		return startTCPPeer(ctx, certificate, expectedClient, binding, mode)
	case quicProfile:
		return startQUICPeer(ctx, certificate, expectedClient, binding, mode)
	default:
		return nil, errIncompatible
	}
}

func startStall(ctx context.Context, profile profileID) (*peerRuntime, error) {
	if profile == tcpProfile {
		return startTCPStall(ctx)
	}
	if profile == quicProfile {
		return startQUICStall(ctx)
	}
	return nil, errIncompatible
}

func newAttempt(material identities, profile profileID, label string, deadline time.Time) carrierAttempt {
	return carrierAttempt{
		AuthorityDigest: sha256.Sum256([]byte("r094-authority")), Profile: profile,
		ExpectedPeer: material.serverID, Certificate: material.client, Deadline: deadline,
		Binding: route.LegBinding{
			NetworkID: sha256.Sum256([]byte("r094-network")), Epoch: 1,
			Digest: sha256.Sum256([]byte("r094-epoch")), AttachmentID: sha256.Sum256([]byte(label)),
			SenderRole: 1, PeerRole: 3, SenderNodeID: material.clientID, PeerNodeID: material.serverID,
			NotAfter: deadline,
		},
	}
}

func reciprocal(value route.LegBinding) route.LegBinding {
	return route.LegBinding{
		NetworkID: value.NetworkID, Epoch: value.Epoch, Digest: value.Digest, AttachmentID: value.AttachmentID,
		SenderRole: value.PeerRole, PeerRole: value.SenderRole, SenderNodeID: value.PeerNodeID,
		PeerNodeID: value.SenderNodeID, NotAfter: value.NotAfter,
	}
}

func futureDeadline(after time.Duration) time.Time {
	return time.Now().Add(after).UTC().Truncate(time.Second)
}

func exchange(carrier *authenticatedCarrier) error {
	if err := writeExact(carrier, []byte(requestBytes)); err != nil {
		return err
	}
	response := make([]byte, len(responseBytes))
	if _, err := io.ReadFull(carrier, response); err != nil {
		return err
	}
	if string(response) != responseBytes {
		return errors.New("carrier returned a noncanonical transcript")
	}
	return nil
}

func writeExact(writer io.Writer, value []byte) error {
	for len(value) != 0 {
		count, err := writer.Write(value)
		if err != nil {
			return err
		}
		if count <= 0 || count > len(value) {
			return io.ErrShortWrite
		}
		value = value[count:]
	}
	return nil
}

func result(name string, profile profileID, expected, observed resultClass, passed bool, module *carrierModule,
	attempt carrierAttempt, bytes int, cleanup bool, started time.Time, note string) caseResult {
	return caseResult{Schema: "ardents-r094-carrier-case-v1", Case: name, Profile: profile, Expected: expected,
		Observed: observed, Passed: passed, AdapterCalls: calls(module), OfferDigest: digestOffer(attempt), Bytes: bytes,
		Cleanup: cleanup, ElapsedMS: time.Since(started).Milliseconds(), Note: note}
}

func failedSetup(name string, profile profileID, expected resultClass, started time.Time, err error) caseResult {
	return caseResult{Schema: "ardents-r094-carrier-case-v1", Case: name, Profile: profile, Expected: expected,
		Observed: classInternal, Passed: false, AdapterCalls: "tcp=0,quic=0", Cleanup: true,
		ElapsedMS: time.Since(started).Milliseconds(), Note: err.Error()}
}

func digestOffer(attempt carrierAttempt) string {
	binding, _ := route.EncodeLegBinding(attempt.Binding)
	value := append([]byte(attempt.Profile), 0)
	value = append(value, attempt.AuthorityDigest[:]...)
	value = append(value, attempt.ExpectedPeer[:]...)
	value = append(value, binding...)
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func selectedOnce(module *carrierModule, profile profileID) bool {
	return module.calls[profile] == 1 && totalCalls(module) == 1
}

func totalCalls(module *carrierModule) int {
	return module.calls[tcpProfile] + module.calls[quicProfile]
}

func calls(module *carrierModule) string {
	return fmt.Sprintf("tcp=%d,quic=%d", module.calls[tcpProfile], module.calls[quicProfile])
}

func joinNote(values ...error) string {
	joined := errors.Join(values...)
	if joined == nil {
		return ""
	}
	return joined.Error()
}
