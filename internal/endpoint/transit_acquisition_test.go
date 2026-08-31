package endpoint

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/route/credential"
)

func TestTransitAcquisitionReconcilesAndBurnsAmbiguousPresentation(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_000, 0).UTC()
	root := filepath.Join(t.TempDir(), "transit-acquisition")
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var signerPublic [32]byte
	copy(signerPublic[:], signer.Public().(ed25519.PublicKey))
	scope := transitAcquisitionScope{NetworkID: acquisitionID(1), Digest: acquisitionID(2), Epoch: 3,
		IssuerNodeID: acquisitionID(4), IssuerPublicKey: acquisitionID(5), IssuerProfileDigest: acquisitionID(6),
		GrantSignerPublicKey: signerPublic, TransitNodeID: acquisitionID(7), TransitRole: route.IntroductionRole,
		NotAfter: now.Add(10 * time.Second)}

	owner, err := openTransitAcquisition(transitAcquisitionConfig{Root: root, Create: true, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	pending, err := owner.begin(scope)
	if err != nil {
		t.Fatal(err)
	}
	if pending.Phase != transitPending || pending.Request.RequestID == [32]byte{} || pending.Request.AttachmentID == [32]byte{} ||
		pending.Request.ClientKeyDigest == [32]byte{} || pending.Certificate.PrivateKey == nil {
		t.Fatalf("pending acquisition = %+v", pending)
	}
	requestID, attachment, keyDigest := pending.Request.RequestID, pending.Request.AttachmentID, pending.Request.ClientKeyDigest
	private := append([]byte(nil), pending.Certificate.PrivateKey.(ed25519.PrivateKey)...)
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	owner, err = openTransitAcquisition(transitAcquisitionConfig{Root: root, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	reconciled, err := owner.begin(scope)
	if err != nil {
		t.Fatal(err)
	}
	if reconciled.Request.RequestID != requestID || reconciled.Request.AttachmentID != attachment ||
		reconciled.Request.ClientKeyDigest != keyDigest || !bytes.Equal(reconciled.Certificate.PrivateKey.(ed25519.PrivateKey), private) {
		t.Fatalf("reconciled acquisition changed its one-use inputs: %+v", reconciled.Request)
	}
	grant := acquisitionGrant(t, scope, reconciled.Request, signer)
	if err := owner.commit(credential.Result{Outcome: credential.Issued, Grant: grant}); err != nil {
		t.Fatal(err)
	}
	ready, err := owner.begin(scope)
	if err != nil || ready.Phase != transitReady || !bytes.Equal(ready.Grant, grant) ||
		!bytes.Equal(ready.Certificate.PrivateKey.(ed25519.PrivateKey), private) {
		t.Fatalf("ready acquisition = %+v, %v", ready, err)
	}
	if _, err := owner.present(scope); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}

	owner, err = openTransitAcquisition(transitAcquisitionConfig{Root: root, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Close() })
	state := owner.stateForTest()
	if state.Phase != transitBurned || len(state.PrivateKey) != 0 || len(state.Certificate) != 0 || len(state.Grant) != 0 {
		t.Fatalf("ambiguous presentation recovery = %+v", state)
	}
}

func TestTransitAcquisitionPersistsFixedTerminalWithoutKey(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_100, 0).UTC()
	root := filepath.Join(t.TempDir(), "transit-acquisition")
	public, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var signerPublic [32]byte
	copy(signerPublic[:], public)
	scope := transitAcquisitionScope{NetworkID: acquisitionID(11), Digest: acquisitionID(12), Epoch: 13,
		IssuerNodeID: acquisitionID(14), IssuerPublicKey: acquisitionID(15), IssuerProfileDigest: acquisitionID(16),
		GrantSignerPublicKey: signerPublic, TransitNodeID: acquisitionID(17), TransitRole: route.IntroductionRole,
		NotAfter: now.Add(10 * time.Second)}
	owner, err := openTransitAcquisition(transitAcquisitionConfig{Root: root, Create: true, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.begin(scope); err != nil {
		t.Fatal(err)
	}
	if err := owner.commit(credential.Result{Outcome: credential.Exhausted}); err != nil {
		t.Fatal(err)
	}
	if _, err := owner.begin(scope); !errors.Is(err, errTransitAcquisitionTerminal) {
		t.Fatalf("terminal acquisition begin error = %v", err)
	}
	state := owner.stateForTest()
	if state.Phase != transitExhausted || len(state.PrivateKey) != 0 || len(state.Certificate) != 0 || len(state.Grant) != 0 {
		t.Fatalf("terminal acquisition retained secret material: %+v", state)
	}
	successor := scope
	successor.Digest, successor.Epoch = acquisitionID(18), scope.Epoch+1
	if next, err := owner.begin(successor); err != nil || next.Phase != transitPending || next.Request.RequestID == state.RequestID {
		t.Fatalf("successor State acquisition = %+v, %v", next, err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTransitAcquisitionRootRequiresExplicitInitializationAndOneOwner(t *testing.T) {
	t.Parallel()
	root := filepath.Join(t.TempDir(), "transit-acquisition")
	clock := func() time.Time { return time.Unix(2_000_000_200, 0).UTC() }
	if _, err := openTransitAcquisition(transitAcquisitionConfig{Root: root, Clock: clock}); err == nil {
		t.Fatal("missing transit acquisition root was initialized implicitly")
	}
	owner, err := openTransitAcquisition(transitAcquisitionConfig{Root: root, Create: true, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := openTransitAcquisition(transitAcquisitionConfig{Root: root, Clock: clock}); err == nil {
		t.Fatal("second transit acquisition owner opened the same root")
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := openTransitAcquisition(transitAcquisitionConfig{Root: root, Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	if err := reopened.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTransitAcquisitionSetSeparatesRoleScopedLifecycles(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_300, 0).UTC()
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var signerPublic [32]byte
	copy(signerPublic[:], signer.Public().(ed25519.PublicKey))
	base := transitAcquisitionScope{NetworkID: acquisitionID(41), Digest: acquisitionID(42), Epoch: 43,
		IssuerNodeID: acquisitionID(44), IssuerPublicKey: acquisitionID(45), IssuerProfileDigest: acquisitionID(46),
		GrantSignerPublicKey: signerPublic, NotAfter: now.Add(10 * time.Second)}
	introductionScope := base
	introductionScope.TransitNodeID, introductionScope.TransitRole = acquisitionID(47), route.IntroductionRole
	responderScope := base
	responderScope.TransitNodeID, responderScope.TransitRole = acquisitionID(48), route.ResponderRole

	root := filepath.Join(t.TempDir(), "transit-acquisitions")
	owners, err := openTransitAcquisitionSet(transitAcquisitionConfig{Root: root, Create: true, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	introduction, err := owners.introduction.begin(introductionScope)
	if err != nil {
		t.Fatal(err)
	}
	responder, err := owners.responder.begin(responderScope)
	if err != nil {
		t.Fatal(err)
	}
	if introduction.Request.TransitRole != route.IntroductionRole || introduction.Request.TransitNodeID != introductionScope.TransitNodeID ||
		responder.Request.TransitRole != route.ResponderRole || responder.Request.TransitNodeID != responderScope.TransitNodeID ||
		introduction.Request.RequestID == responder.Request.RequestID || introduction.Request.AttachmentID == responder.Request.AttachmentID {
		t.Fatalf("role-scoped acquisition requests overlap: introduction=%+v responder=%+v", introduction.Request, responder.Request)
	}
	if err := owners.introduction.commit(credential.Result{Outcome: credential.Issued,
		Grant: acquisitionGrant(t, introductionScope, introduction.Request, signer)}); err != nil {
		t.Fatal(err)
	}
	if err := owners.responder.commit(credential.Result{Outcome: credential.Exhausted}); err != nil {
		t.Fatal(err)
	}
	if ready, err := owners.introduction.begin(introductionScope); err != nil || ready.Phase != transitReady {
		t.Fatalf("Introduction lifecycle did not remain ready: %+v, %v", ready, err)
	}
	if _, err := owners.responder.begin(responderScope); !errors.Is(err, errTransitAcquisitionTerminal) {
		t.Fatalf("Responder lifecycle did not retain its independent terminal outcome: %v", err)
	}
	if err := owners.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestTransitAcquisitionReadsAcceptedV1IntroductionStateAsV2(t *testing.T) {
	t.Parallel()
	now := time.Unix(2_000_000_400, 0).UTC()
	root := filepath.Join(t.TempDir(), "transit-acquisition")
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	var signerPublic [32]byte
	copy(signerPublic[:], signer.Public().(ed25519.PublicKey))
	scope := transitAcquisitionScope{NetworkID: acquisitionID(51), Digest: acquisitionID(52), Epoch: 53,
		IssuerNodeID: acquisitionID(54), IssuerPublicKey: acquisitionID(55), IssuerProfileDigest: acquisitionID(56),
		GrantSignerPublicKey: signerPublic, TransitNodeID: acquisitionID(57), TransitRole: route.IntroductionRole,
		NotAfter: now.Add(10 * time.Second)}
	owner, err := openTransitAcquisition(transitAcquisitionConfig{Root: root, Create: true, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := owner.begin(scope); err != nil {
		t.Fatal(err)
	}
	state := owner.stateForTest()
	previous := transitAcquisitionStateV1{Schema: transitAcquisitionSchemaV1, Phase: state.Phase,
		NetworkID: state.NetworkID, Digest: state.Digest, Epoch: state.Epoch, IssuerNodeID: state.IssuerNodeID,
		IssuerPublicKey: state.IssuerPublicKey, IssuerProfileDigest: state.IssuerProfileDigest,
		GrantSignerPublicKey: state.GrantSignerPublicKey, IntroductionNodeID: state.TransitNodeID,
		NotAfter: state.NotAfter, RequestID: state.RequestID, AttachmentID: state.AttachmentID,
		ClientKeyDigest: state.ClientKeyDigest, Certificate: state.Certificate, PrivateKey: state.PrivateKey}
	raw, err := json.Marshal(previous)
	if err != nil {
		t.Fatal(err)
	}
	if err := replaceTransitAcquisitionState(root, append(raw, '\n')); err != nil {
		t.Fatal(err)
	}
	if err := owner.Close(); err != nil {
		t.Fatal(err)
	}
	owner, err = openTransitAcquisition(transitAcquisitionConfig{Root: root, Clock: func() time.Time { return now }})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	reconciled, err := owner.begin(scope)
	if err != nil || reconciled.Request.TransitRole != route.IntroductionRole || reconciled.Request.TransitNodeID != scope.TransitNodeID {
		t.Fatalf("accepted v1 state did not reconcile as role-scoped v2: %+v, %v", reconciled.Request, err)
	}
}

func acquisitionGrant(t *testing.T, scope transitAcquisitionScope, request credential.Request, private ed25519.PrivateKey) []byte {
	t.Helper()
	raw, err := route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(private.Public().(ed25519.PublicKey)), GrantID: acquisitionID(31),
		NetworkID: scope.NetworkID, Digest: scope.Digest, AttachmentID: request.AttachmentID,
		TransitNodeID: scope.TransitNodeID, ClientKeyDigest: request.ClientKeyDigest,
		Epoch: scope.Epoch, TransitRole: scope.TransitRole, NotAfter: scope.NotAfter}, private)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func acquisitionID(marker byte) [32]byte { return [32]byte{marker} }
