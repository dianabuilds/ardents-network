//go:build h4_8_a11

package service_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/alphacontrol"
	"github.com/dianabuilds/ardents-network/internal/alphacontrol/inspection"
	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/endpoint/enrollment"
	"github.com/dianabuilds/ardents-network/internal/network/source"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/release"
	"github.com/dianabuilds/ardents-network/internal/route"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

var h48A11ExpiryBoundary = time.Date(2030, time.January, 2, 3, 4, 6, 0, time.UTC)

func TestH48A11DeterministicExpiryBoundaries(t *testing.T) {
	before, boundary := h48A11ExpiryBoundary.Add(-time.Second), h48A11ExpiryBoundary
	t.Run("ExactCandidateControlBoundary", func(t *testing.T) {
		assertH48A11ExactCandidateExpiry(t)
	})
	t.Logf("A11_EXPIRY_BOUNDARY owner=network-state before=%s at=%s", before.Format(time.RFC3339), boundary.Format(time.RFC3339))
	t.Run("NetworkStateCurrentDutyAndFreshWork", func(t *testing.T) {
		assertH48A11StateExpiry(t, before, boundary)
	})
	t.Logf("A11_EXPIRY_BOUNDARY owner=alpha-control before=%s at=%s", before.Format(time.RFC3339), boundary.Format(time.RFC3339))
	t.Run("ACA1ComponentsAndPersistedFloor", func(t *testing.T) {
		assertH48A11ControlExpiry(t, before, boundary)
	})
	t.Logf("A11_EXPIRY_BOUNDARY owner=service-credential before=%s at=%s", before.Format(time.RFC3339), boundary.Format(time.RFC3339))
	t.Run("ServiceCredentialAndFreshWork", func(t *testing.T) {
		assertH48A11ServiceCredentialExpiry(t, before, boundary)
	})
	if !t.Failed() {
		t.Logf("A11_EXPIRY_RESULT schema=ardents-h4-8-a11-expiry-v1 owners=3 before=%s at=%s status=accepted",
			before.Format(time.RFC3339), boundary.Format(time.RFC3339))
	}
}

func assertH48A11StateExpiry(t *testing.T, before, boundary time.Time) {
	t.Helper()
	_, authority, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	roles := make(map[string]referenceC2StateRecord, 5)
	for index, role := range []string{"destination-resolution", "initiator", "introduction", "rendezvous", "responder"} {
		marker := byte(index + 1)
		roles[role] = referenceC2StateRecord{role: role, nodeID: referenceC2ID(marker),
			material: referenceC2Certificate(t, int64(marker), "a11-expiry-"+role),
			endpoint: "127.0.0.1:" + []string{"49101", "49102", "49103", "49104", "49105"}[index], family: "a11-" + role}
	}
	fixture := newReferenceC2StateFixture(t, boundary.Add(-time.Minute), boundary, authority, roles)
	public := authority.Public().(ed25519.PublicKey)
	injected := fixture.now
	opened, err := state.Open(state.Config{Root: filepath.Join(t.TempDir(), "state"), NetworkID: fixture.network,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1,
		Clock: func() time.Time { return injected }, AcceptedProfile: route.Profile,
		Source: source.Config{MaterialIndex: fixture.roles["rendezvous"].materializationIndex}})
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	accepted, err := opened.Accept(context.Background(), fixture.raw, fixture.inputs, [][]byte{fixture.materials["rendezvous"]})
	if err != nil || accepted.Freshness != "fresh" {
		t.Fatalf("State acceptance before expiry: freshness=%q err=%v", accepted.Freshness, err)
	}
	injected = before
	duty, err := opened.CurrentNodeDuty()
	if err != nil || !duty.DutyFresh() || !duty.DutyValidUntil().Equal(boundary) {
		t.Fatalf("current duty before expiry: fresh=%v not_after=%s err=%v", duty.DutyFresh(), duty.DutyValidUntil(), err)
	}
	view, err := opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	if _, available := view.Epoch(before, boundary); !available {
		t.Fatal("State refused fresh work whose exact bounded window ended at NotAfter")
	}

	injected = boundary
	duty, err = opened.CurrentNodeDuty()
	if err != nil || duty.DutyFresh() {
		t.Fatalf("State duty at exact NotAfter: fresh=%v err=%v", duty.DutyFresh(), err)
	}
	view, err = opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	if _, available := view.Epoch(boundary, boundary.Add(time.Second)); available {
		t.Fatal("State admitted fresh work at exact NotAfter")
	}
	injected = boundary.Add(time.Second)
	view, err = opened.CurrentResolution()
	if err != nil {
		t.Fatal(err)
	}
	if _, available := view.Epoch(injected, injected.Add(time.Second)); available {
		t.Fatal("State admitted fresh work after NotAfter")
	}
	for _, at := range []time.Time{boundary, boundary.Add(time.Second)} {
		fresh, openErr := state.Open(state.Config{Root: filepath.Join(t.TempDir(), at.Format("150405")), NetworkID: fixture.network,
			Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1, Now: at,
			AcceptedProfile: route.Profile, Source: source.Config{MaterialIndex: fixture.roles["rendezvous"].materializationIndex}})
		if openErr != nil {
			t.Fatal(openErr)
		}
		_, acceptErr := fresh.Accept(context.Background(), fixture.raw, fixture.inputs, [][]byte{fixture.materials["rendezvous"]})
		if closeErr := fresh.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		if acceptErr == nil {
			t.Fatalf("fresh State root accepted expired Epoch at %s", at.Format(time.RFC3339))
		}
	}
}

type h48A11ControlFixture struct {
	accepted, lower, conflict enrollment.Request
	references                [3]alphacontrol.Component
	roots                     [3]ed25519.PublicKey
	components                [3][]byte
	network, networkDigest    [32]byte
}

func assertH48A11ControlExpiry(t *testing.T, before, boundary time.Time) {
	t.Helper()
	fixture := newH48A11ControlFixture(t, before, boundary)
	root := filepath.Join(t.TempDir(), "persisted-inspection")
	report, err := inspection.Inspect(context.Background(), inspection.Config{Root: root, Enrollment: fixture.accepted, At: before})
	if err != nil {
		t.Fatalf("ACA1 inspection at NotAfter-1s: %v", err)
	}
	if report.Inspection.Catalog != alphacontrol.OutcomeAccepted || report.CatalogGeneration != 2 || !report.CatalogNotAfter.Equal(boundary) ||
		report.Release != string(release.OutcomeReleaseAccepted) || report.NetworkID != fixture.network || report.NetworkEpoch != 1 ||
		report.NetworkDigest != fixture.networkDigest || report.NetworkProfile != "h3-role-probe-v1" {
		t.Fatalf("accepted ACA1 identity is incomplete: catalog=%q generation=%d release=%q network_epoch=%d profile=%q",
			report.Inspection.Catalog, report.CatalogGeneration, report.Release, report.NetworkEpoch, report.NetworkProfile)
	}
	for index, component := range report.Inspection.Components {
		if component.Class != alphacontrol.ComponentClass(index+1) || component.Outcome != alphacontrol.OutcomeAccepted ||
			!report.ComponentDetails[index].NotAfter.Equal(boundary) {
			t.Fatalf("component %d before expiry: class=%d outcome=%q not_after=%s", index, component.Class,
				component.Outcome, report.ComponentDetails[index].NotAfter.Format(time.RFC3339))
		}
	}

	lower, err := inspection.Inspect(context.Background(), inspection.Config{Root: root, Enrollment: fixture.lower, At: before})
	if err == nil || lower.Inspection.Catalog != alphacontrol.OutcomeLowerFloor {
		t.Fatalf("lower ACA1 generation after persisted floor: catalog=%q err=%v", lower.Inspection.Catalog, err)
	}
	conflict, err := inspection.Inspect(context.Background(), inspection.Config{Root: root, Enrollment: fixture.conflict, At: before})
	if err == nil || conflict.Inspection.Catalog != alphacontrol.OutcomeConflict {
		t.Fatalf("same-generation ACA1 conflict after persisted floor: catalog=%q err=%v", conflict.Inspection.Catalog, err)
	}

	expired, err := inspection.Inspect(context.Background(), inspection.Config{Root: root, Enrollment: fixture.accepted, At: boundary})
	if err == nil || expired.Inspection.Catalog != alphacontrol.OutcomeInvalid {
		t.Fatalf("persisted ACA1 repeat at exact NotAfter: catalog=%q err=%v", expired.Inspection.Catalog, err)
	}
	for index := range fixture.components {
		if outcome := alphacontrol.VerifyComponent(fixture.references[index], fixture.components[index], fixture.roots[index], boundary); outcome != alphacontrol.OutcomeExpired {
			t.Fatalf("component %d at exact NotAfter = %q, want %q", index+1, outcome, alphacontrol.OutcomeExpired)
		}
	}
	fresh, err := inspection.Inspect(context.Background(), inspection.Config{Root: filepath.Join(t.TempDir(), "fresh-inspection"), Enrollment: fixture.accepted, At: boundary})
	if err == nil || fresh.Inspection.Catalog != alphacontrol.OutcomeInvalid {
		t.Fatalf("fresh-root ACA1 inspection at exact NotAfter: catalog=%q err=%v", fresh.Inspection.Catalog, err)
	}
}

func newH48A11ControlFixture(t *testing.T, before, boundary time.Time) h48A11ControlFixture {
	t.Helper()
	vector := h48A11ReleaseVector(t)
	inputs := release.Inputs{RootBytes: vector["1.root.json"], Files: map[string][]byte{
		release.MetadataURL("timestamp.json"):  vector["timestamp.json"],
		release.MetadataURL("1.snapshot.json"): vector["1.snapshot.json"],
		release.MetadataURL("1.targets.json"):  vector["1.targets.json"],
	}, TargetPath: "ardents/windows-amd64/application", Artifact: vector["ardents-windows-amd64"],
		Local: release.LocalEnvironment{Environment: "h3-test", Network: "ardents-h3-test-1", Platform: "windows-amd64", Architecture: "amd64", RefTime: before}}
	verifier, err := release.Open(filepath.Join(t.TempDir(), "release-seed"))
	if err != nil {
		t.Fatal(err)
	}
	decision := verifier.Evaluate(context.Background(), inputs)
	if closeErr := verifier.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil || decision.Outcome != release.OutcomeReleaseAccepted || len(decision.Digest) != sha256.Size {
		t.Fatalf("release seed at NotAfter-1s: outcome=%q err=%v", decision.Outcome, err)
	}
	var releaseDigest [32]byte
	copy(releaseDigest[:], decision.Digest)
	releaseBody, err := inspection.EncodeReleaseEvidence(inspection.ReleaseEvidence{ArtifactDigest: releaseDigest, TargetPath: decision.Path,
		ReleaseIdentity: decision.ReleaseIdentity, BuildIdentity: decision.BuildIdentity, ProtocolPhase: decision.ProtocolPhase, BuildState: decision.BuildState})
	if err != nil {
		t.Fatal(err)
	}
	networkPublic, networkPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	network := sha256.Sum256([]byte("h4-8-a11-expiry-network"))
	epoch, networkDigest := h48A11SignedEmptyEpoch(network, boundary, networkPrivate)
	networkBody, err := inspection.EncodeNetworkEvidence(inspection.NetworkEvidence{NetworkID: network, EpochDigest: networkDigest,
		Profile: "h3-role-probe-v1", Threshold: 1, Authorities: []ed25519.PublicKey{networkPublic}, Epoch: epoch})
	if err != nil {
		t.Fatal(err)
	}
	compatibilityBody, err := inspection.EncodeCompatibilityEvidence(inspection.CompatibilityEvidence{ReleaseDigest: releaseDigest,
		ReleaseBuildIdentity: decision.BuildIdentity, ProtocolPhase: decision.ProtocolPhase, NetworkDigest: networkDigest,
		NetworkEpoch: 1, NetworkProfile: "h3-role-probe-v1"})
	if err != nil {
		t.Fatal(err)
	}

	fixture := h48A11ControlFixture{network: network, networkDigest: networkDigest}
	bodies := [3][]byte{releaseBody, networkBody, compatibilityBody}
	for index, body := range bodies {
		public, private, keyErr := ed25519.GenerateKey(rand.Reader)
		if keyErr != nil {
			t.Fatal(keyErr)
		}
		class := alphacontrol.ComponentClass(index + 1)
		component, signErr := alphacontrol.SignComponent(alphacontrol.ComponentStatement{Class: class, Generation: 2,
			NotBefore: boundary.Add(-10 * time.Minute), NotAfter: boundary, Body: body}, private)
		if signErr != nil {
			t.Fatal(signErr)
		}
		fixture.roots[index], fixture.components[index] = public, component
		fixture.references[index] = alphacontrol.Component{Class: class, RootID: sha256.Sum256(public), Generation: 2,
			NotAfter: boundary, Size: uint32(len(component)), Digest: sha256.Sum256(component)}
	}
	disclosurePublic, disclosurePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sign := func(generation uint64, notBefore time.Time) []byte {
		raw, signErr := alphacontrol.Sign(alphacontrol.Catalog{Cohort: "a11-expiry", Generation: generation,
			NotBefore: notBefore, NotAfter: boundary, Components: fixture.references}, disclosurePrivate)
		if signErr != nil {
			t.Fatal(signErr)
		}
		return raw
	}
	common := map[string][]byte{}
	for name, raw := range vector {
		common[name] = raw
	}
	common["catalog.pub"] = disclosurePublic
	for index, name := range []string{"release", "network", "compatibility"} {
		common[name+".ac1"], common[name+".pub"] = fixture.components[index], fixture.roots[index]
	}
	fixture.accepted = h48A11EnrollmentBundle(t, common, sign(2, boundary.Add(-10*time.Minute)), before)
	fixture.lower = h48A11EnrollmentBundle(t, common, sign(1, boundary.Add(-10*time.Minute)), before)
	fixture.conflict = h48A11EnrollmentBundle(t, common, sign(2, boundary.Add(-9*time.Minute)), before)
	return fixture
}

func h48A11ReleaseVector(t *testing.T) map[string][]byte {
	t.Helper()
	_, here, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate A11 expiry test")
	}
	directory := filepath.Join(filepath.Dir(here), "..", "..", "..", "internal", "release", "testdata", "r049-public-vector-v1")
	read := func(name string) []byte {
		raw, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	return map[string][]byte{"1.root.json": read("root.json"), "timestamp.json": read("timestamp.json"),
		"1.snapshot.json": read("1.snapshot.json"), "1.targets.json": read("1.targets.json"),
		"ardents-windows-amd64": read("artifact.bin")}
}

func h48A11SignedEmptyEpoch(network [32]byte, boundary time.Time, signer ed25519.PrivateKey) ([]byte, [32]byte) {
	inputRoot, viewRoot, rejectedRoot := sha256.Sum256([]byte{0x10}), sha256.Sum256([]byte{0x11}), sha256.Sum256([]byte{0x12})
	buffer := new(bytes.Buffer)
	buffer.WriteString("AREP")
	buffer.WriteByte(1)
	buffer.Write(network[:])
	referenceC2U64(buffer, 1)
	buffer.Write(make([]byte, 32))
	referenceC2I64(buffer, boundary.Add(-time.Minute).Unix())
	referenceC2I64(buffer, boundary.Unix())
	referenceC2U32(buffer, 0)
	referenceC2Text(buffer, "h3-role-probe-v1")
	buffer.Write(inputRoot[:])
	buffer.Write(viewRoot[:])
	referenceC2U32(buffer, 0)
	buffer.Write(rejectedRoot[:])
	referenceC2U32(buffer, 0)
	buffer.Write(make([]byte, 32))
	referenceC2Text(buffer, "ardents-h3-role-domain-v1")
	referenceC2U32(buffer, 0)
	referenceC2U32(buffer, 0)
	referenceC2U16(buffer, 0)
	referenceC2U16(buffer, 0)
	referenceC2U32(buffer, 0)
	buffer.WriteByte(1)
	referenceC2Text(buffer, "alpha")
	referenceC2U16(buffer, 0)
	referenceC2U32(buffer, 0)
	unsigned := append([]byte(nil), buffer.Bytes()...)
	digest := sha256.Sum256(unsigned)
	public := signer.Public().(ed25519.PublicKey)
	id := sha256.Sum256(public)
	buffer.WriteByte(1)
	buffer.Write(id[:])
	buffer.Write(ed25519.Sign(signer, digest[:]))
	return buffer.Bytes(), digest
}

func h48A11EnrollmentBundle(t *testing.T, common map[string][]byte, catalog []byte, at time.Time) enrollment.Request {
	t.Helper()
	bundle := t.TempDir()
	descriptor := strings.Join([]string{"schema=ardents-closed-alpha-enrollment-v1", "cohort=a11-expiry",
		"release=a11-expiry-candidate", "platform=windows-amd64", "environment=h3-test", "network=ardents-h3-test-1",
		"target_path=ardents/windows-amd64/application", "artifact=ardents-windows-amd64", "trusted_root=1.root.json",
		"control_catalog=catalog.ac1", "disclosure_root=catalog.pub", "control_release=release.ac1", "control_network=network.ac1",
		"control_compatibility=compatibility.ac1", "control_release_root=release.pub", "control_network_root=network.pub",
		"control_compatibility_root=compatibility.pub"}, "\n") + "\n"
	files := make(map[string][]byte, len(common)+2)
	for name, raw := range common {
		files[name] = append([]byte(nil), raw...)
	}
	files["RELEASE"], files["catalog.ac1"] = []byte(descriptor), append([]byte(nil), catalog...)
	names := make([]string, 0, len(files))
	for name, raw := range files {
		mode := os.FileMode(0o600)
		if name == "ardents-windows-amd64" {
			mode = 0o700
		}
		if err := os.WriteFile(filepath.Join(bundle, name), raw, mode); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	sort.Strings(names)
	var manifest strings.Builder
	for _, name := range names {
		fmt.Fprintf(&manifest, "%x  %s\n", sha256.Sum256(files[name]), name)
	}
	manifestBytes := []byte(manifest.String())
	if err := os.WriteFile(filepath.Join(bundle, "SHA256SUMS"), manifestBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	pin := sha256.Sum256(manifestBytes)
	return enrollment.Request{BundleRoot: bundle, ExecutablePath: filepath.Join(bundle, "ardents-windows-amd64"),
		Pin: enrollment.Pin{Cohort: "a11-expiry", Release: "a11-expiry-candidate", Platform: "windows-amd64",
			ManifestSHA256: hex.EncodeToString(pin[:])}, Environment: "h3-test", Network: "ardents-h3-test-1",
		TargetPath: "ardents/windows-amd64/application", Architecture: "amd64", ReferenceTime: at}
}

func assertH48A11ServiceCredentialExpiry(t *testing.T, before, boundary time.Time) {
	t.Helper()
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	introductionPublic, introductionPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	networkID := sha256.Sum256([]byte("h4-8-a11-expiry-service-network"))
	brokerID := sha256.Sum256([]byte("h4-8-a11-expiry-service-broker"))
	connectionPrincipal := sha256.Sum256([]byte("h4-8-a11-expiry-connection-principal"))
	administrationPrincipal := sha256.Sum256([]byte("h4-8-a11-expiry-administration-principal"))
	var instance [32]byte
	copy(instance[:], instancePublic)
	credential, err := (endpointapi.Credential{InstancePublic: instance,
		IntroductionHPKEPublic: sha256.Sum256([]byte("h4-8-a11-expiry-introduction-hpke")), Generation: 1,
		NotBefore: boundary.Add(-time.Minute).Unix(), NotAfter: boundary.Unix(), NetworkID: networkID, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	injected := before
	publisher, err := endpointapi.New(endpointapi.Setup{NetworkID: networkID, BrokerID: brokerID, AuthorityPublic: authorityPublic,
		IntroductionPublic: introductionPublic, ConnectionPrincipal: connectionPrincipal, AdministrationPrincipal: administrationPrincipal,
		PublicationRoot: t.TempDir(), Clock: func() time.Time { return injected }})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if closeErr := publisher.Close(); closeErr != nil {
			t.Errorf("close expiry publisher: %v", closeErr)
		}
	}()
	admin, err := publisher.Admit(administrationPrincipal, broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	published, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{Principal: administrationPrincipal,
		Capability: admin, Credential: credential, InstanceSigner: instancePrivate,
		IntroductionAcknowledgement: h48A11IntroductionAcknowledgement(credential, introductionPrivate, brokerID), At: before})
	if err != nil || published.Class != "published" || published.AuthenticatedTarget != credential.Target || published.Generation != 1 {
		t.Fatalf("Service Credential publication at NotAfter-1s: class=%q generation=%d err=%v", published.Class, published.Generation, err)
	}
	if current, decodeErr := publication.Decode(published.Record, authorityPublic, networkID, before); decodeErr != nil ||
		current.Credential.Target != credential.Target || current.Credential.Generation != 1 {
		t.Fatalf("Service Credential decode at NotAfter-1s: target=%x generation=%d err=%v",
			current.Credential.Target, current.Credential.Generation, decodeErr)
	}
	if _, decodeErr := publication.Decode(published.Record, authorityPublic, networkID, boundary); decodeErr == nil {
		t.Fatal("Service Credential remained current at exact NotAfter")
	}

	injected = boundary
	admin, err = publisher.Admit(administrationPrincipal, broker.Administration)
	if err != nil {
		t.Fatal(err)
	}
	if result, publishErr := publisher.Publish(context.Background(), endpointapi.PublicationRequest{Principal: administrationPrincipal,
		Capability: admin, Credential: credential, InstanceSigner: instancePrivate,
		IntroductionAcknowledgement: h48A11IntroductionAcknowledgement(credential, introductionPrivate, brokerID), At: boundary}); publishErr == nil || result.Class != "service target authentication failure" {
		t.Fatalf("fresh Service publication at exact NotAfter: class=%q err=%v", result.Class, publishErr)
	}
	connection, err := publisher.Admit(connectionPrincipal, broker.Connection)
	if err != nil {
		t.Fatal(err)
	}
	routeSide, routePeer := net.Pipe()
	applicationSide, applicationPeer := net.Pipe()
	defer routeSide.Close()
	defer routePeer.Close()
	defer applicationSide.Close()
	defer applicationPeer.Close()
	result, acceptErr := publisher.Accept(context.Background(), endpointapi.InboundConnectionRequest{Principal: connectionPrincipal,
		Capability: connection, Route: routeSide, Application: applicationSide, BytesEachDirection: 1, At: boundary})
	if acceptErr == nil || result.Class != "service unavailable" {
		t.Fatalf("fresh Service Connection work at exact NotAfter: class=%q err=%v", result.Class, acceptErr)
	}
	assertH48A11ReachabilityExpiry(t, published.Record, authorityPublic, introductionPublic, instancePrivate, networkID, credential.Target, before, boundary)
}

func h48A11IntroductionAcknowledgement(credential endpointapi.Credential, private ed25519.PrivateKey, brokerID [32]byte) []byte {
	body := make([]byte, 149)
	copy(body[:4], "ARIA")
	body[4] = 1
	copy(body[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(body[37:45], credential.Generation)
	binary.BigEndian.PutUint64(body[45:53], uint64(credential.NotAfter))
	copy(body[53:85], credential.NetworkID[:])
	copy(body[85:117], brokerID[:])
	body[117] = 1
	signature := ed25519.Sign(private, append([]byte("ardents-service-introduction-ack-v1\x00"), body...))
	return append(body, signature...)
}
