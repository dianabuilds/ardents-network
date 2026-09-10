//go:build linux

package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/route"
)

type issuerReservationObservation struct {
	RequestID, RequestDigest, PermissionID [32]byte
	Window                                 time.Time
	Class                                  uint8
	Kind                                   closedIssuanceKind
	Count                                  uint16
}
type issuerStateObservation struct {
	Phase        string
	Network      [32]byte
	Profile      state.ClosedProfileView
	Closed       bool
	Material     []byte
	Reservations []issuerReservationObservation
	Durable      map[string][]byte
}

// Caller has joined the actual issuance exchange. Snapshot mutable issuer
// fields under its mutex. Raw material is synthetic fixture secret evidence,
// written only to the explicitly selected local private evidence directory.
func observeClosedIssuer(t *testing.T, issuer *ClosedTokenIssuer, phase string) issuerStateObservation {
	t.Helper()
	issuer.mu.Lock()
	defer issuer.mu.Unlock()
	material, err := encodeClosedIssuerMaterial(issuer.material)
	if err != nil {
		t.Fatal(err)
	}
	snapshot := issuerStateObservation{Phase: phase, Network: issuer.network, Profile: issuer.profile, Closed: issuer.closed, Material: material, Durable: make(map[string][]byte)}
	if issuer.ledger.failure != nil {
		t.Fatal(issuer.ledger.failure)
	}
	for _, entry := range issuer.ledger.reservations {
		snapshot.Reservations = append(snapshot.Reservations, issuerReservationObservation{entry.requestID, entry.requestDigest, entry.permissionID, entry.window, entry.class, entry.kind, entry.count})
	}
	total := int64(0)
	err = filepath.WalkDir(issuer.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("nonregular issuer evidence file %s", entry.Name())
		}
		total += info.Size()
		if total > 2<<20 {
			t.Fatal("issuer evidence exceeds fixture bound")
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(issuer.root, path)
		if err != nil {
			return err
		}
		snapshot.Durable[filepath.ToSlash(name)] = raw
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// Both permissions are actually verified and durably debited by ServeBootstrap.
// Provisioning and the lower channel are fixtures: this does not claim two
// Endpoint processes, a TLS transcript, complete transient state, or full P3.
func checkClosedIssuerSeparatePermissionObservation(t *testing.T, issuer *ClosedTokenIssuer, profile state.ClosedProfileView,
	original Permission, authority ed25519.PrivateKey, tokenContext ClosedTokenContext, now time.Time, originalOperation, originalReply []byte) {
	t.Helper()
	before := observeClosedIssuer(t, issuer, "after original permission and exhausted retry")
	if len(before.Reservations) != 1 || before.Reservations[0].PermissionID != original.PermissionID || before.Reservations[0].Count != 2 {
		t.Fatal("observation missed original debit or counted exhausted attempt")
	}
	holderPublic, holder, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(holder)
	separate := original
	if _, err := rand.Read(separate.PermissionID[:]); err != nil {
		t.Fatal(err)
	}
	copy(separate.HolderKey[:], holderPublic)
	separate.Maxima = [3]uint32{1, 0, 0}
	copy(separate.Signature[:], ed25519.Sign(authority, permissionTranscript(separate)))
	if separate.PermissionID == original.PermissionID || separate.HolderKey == original.HolderKey {
		t.Fatal("independent permission reused scoped identity")
	}
	pending, err := PrepareClosedTokenBatch(ClosedTokenBatchConfig{Profile: profile, Contexts: []ClosedTokenContext{tokenContext}, Permission: separate, HolderKey: holder, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	defer pending.Discard()
	var nonce [32]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	requestBytes := pending.Request()
	operation, err := route.EncodeClosedIssuanceRequest(nonce, requestBytes)
	if err != nil {
		t.Fatal(err)
	}
	reply := serveClosedIssuerBootstrap(t, issuer, profile, now, operation)
	tokens, err := pending.FinalizeTerminalOperation(nonce, reply)
	if err != nil || len(tokens) != 1 {
		t.Fatalf("separate permission did not issue: %v", err)
	}
	request, err := DecodeClosedTokenBatch(requestBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := VerifyClosedToken(tokenContext, request.SPKI[:], tokens[0]); err != nil {
		t.Fatal(err)
	}
	after := observeClosedIssuer(t, issuer, "after independent permission")
	if len(after.Reservations) != 2 || after.Reservations[0] != before.Reservations[0] || after.Reservations[1].PermissionID != separate.PermissionID || after.Reservations[1].Count != 1 {
		t.Fatal("separate permission changed or merged original debit")
	}
	retried := serveClosedIssuerBootstrap(t, issuer, profile, now, operation)
	final := observeClosedIssuer(t, issuer, "after exact independent retry")
	if !bytes.Equal(reply, retried) || len(final.Reservations) != 2 || final.Reservations[0] != after.Reservations[0] || final.Reservations[1] != after.Reservations[1] {
		t.Fatal("exact retry changed issuer result or scoped debit")
	}
	evidence := struct {
		OriginalOperation, OriginalReply, SeparateOperation, SeparateReply, RetriedReply []byte
		States                                                                           []issuerStateObservation
		Limit                                                                            string
	}{originalOperation, originalReply, operation, reply, retried, []issuerStateObservation{before, after, final},
		"actual issuer request/results, material and durable reservation state; offline permission and net.Pipe/bootstrap channel fixtures; no complete transient/TLS/Endpoint or P3 verdict"}
	if output := os.Getenv("ARDENTS_ISSUER_OBSERVATIONS"); output != "" {
		if !filepath.IsAbs(output) {
			t.Fatal("issuer evidence output must be absolute")
		}
		raw, err := json.Marshal(evidence)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, "issuer-permission-observation.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Log("observed two separately verified permissions, retained debits and exact retry; incomplete P3")
}
