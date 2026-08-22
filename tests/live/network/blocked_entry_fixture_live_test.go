//go:build live

package network_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/hex"
	"encoding/pem"
	"math/big"
	"path/filepath"
	"testing"
	"time"

	localroles "github.com/dianabuilds/ardents-network/internal/network/duty"
	"github.com/dianabuilds/ardents-network/internal/network/state"
	"github.com/dianabuilds/ardents-network/internal/serviceconn"
)

type blockedEntryFixture struct {
	root, clientBinary, serverBinary string
	target, manifest                 [32]byte
}

type blockedBridgeNetwork struct {
	root            string
	snapshot        state.Snapshot
	authorityPublic ed25519.PublicKey
	nodePrivate     ed25519.PrivateKey
	domainProof     []byte
}

func newBlockedEntryFixture(t *testing.T, clientBinary, serverBinary string) blockedEntryFixture {
	t.Helper()
	base := newLiveFixture(t)
	root := filepath.Join(base.root, "blocked-entry")
	for _, role := range []string{"endpoint", "bridge", "probe", "initiator", "introduction", "rendezvous",
		"responder", "publisher", "client-service", "publisher-service", "client-app", "publisher-app", "pressure",
		"capacity-probe", "direct"} {
		mustMkdir(t, filepath.Join(root, "input", role))
		mustMkdirShared(t, filepath.Join(root, "sync", role))
	}
	// State is private to each role but survives a process/container restart
	// inside one fixture. The fixture root itself is unique and removed after
	// the cell, so it cannot carry state into another episode.
	for _, role := range []string{"endpoint", "bridge"} {
		mustMkdirShared(t, filepath.Join(root, "state", role))
	}
	writeBlockedRouteInputs(t, root, base)
	target := writeBlockedServiceInputs(t, root, base)
	writeBlockedBridgeInputs(t, root, base)
	return blockedEntryFixture{root: root, clientBinary: clientBinary, serverBinary: serverBinary,
		target: target, manifest: base.manifest}
}
func writeBlockedRouteInputs(t *testing.T, root string, value liveFixture) {
	t.Helper()
	roles := []string{"initiator", "introduction", "rendezvous", "responder"}
	for index, role := range roles {
		directory := filepath.Join(root, "input", role)
		copyLiveIdentity(t, value, index, directory)
		upstream := value.identities[5].public
		if index > 0 {
			upstream = value.identities[index-1].public
		}
		nextID, nextAddress, nextPin := value.publisherID, value.addresses[4], value.identities[4].public
		if index < 3 {
			nextID, nextAddress, nextPin = value.plan.Positions[index+1].NodeID,
				value.addresses[index+1], value.identities[index+1].public
		}
		plan := map[string]any{"Role": role, "ManifestDigest": liveHex(value.manifest),
			"NetworkID": liveHex(value.network), "EpochDigest": liveHex(value.epochDigest),
			"NodeID": liveHex(value.plan.Positions[index].NodeID), "Listen": value.addresses[index],
			"Certificate": "/run/secure/cert.pem", "Key": "/run/secure/key.pem",
			"UpstreamPin": liveHex(upstream), "NextNodeID": liveHex(nextID), "Next": nextAddress,
			"NextPin": liveHex(nextPin), "Deadline": "15s", "Lifetime": "15m",
			"MaximumAttachments": 1, "AttachmentTarget": 1, "ResourceProfile": "h3-np1-v1"}
		if role == "introduction" {
			plan["AcknowledgementSocket"] = "/run/ardents/introduction-ack/ack.sock"
			plan["AcknowledgementKey"] = "/run/secure/introduction-ack.hex"
			writeLiveFile(t, filepath.Join(directory, "introduction-ack.hex"),
				[]byte(hex.EncodeToString(deterministicLiveKeyBytes(0xb2))))
		}
		writeLivePlan(t, directory, "plan", plan)
	}
	publisher := filepath.Join(root, "input", "publisher")
	copyLiveIdentity(t, value, 4, publisher)
	writeLivePlan(t, publisher, "plan", map[string]any{"Role": "publisher",
		"ManifestDigest": liveHex(value.manifest), "NetworkID": liveHex(value.network),
		"EpochDigest": liveHex(value.epochDigest), "NodeID": liveHex(value.publisherID),
		"Listen": value.addresses[4], "Certificate": "/run/secure/cert.pem", "Key": "/run/secure/key.pem",
		"UpstreamPin": liveHex(value.identities[3].public), "RawAttachment": true,
		"Stream": "/run/ardents/publisher-route/route.sock", "Deadline": "15s", "Lifetime": "15m",
		"MaximumAttachments": 1, "AttachmentTarget": 1, "ResourceProfile": "h3-np1-v1"})
	endpoint := filepath.Join(root, "input", "endpoint")
	copyLiveIdentity(t, value, 5, endpoint)
	copyTree(t, value.stateRoot, filepath.Join(endpoint, "route-state"))
	authority := value.authority.Public().(ed25519.PublicKey)
	writeLivePlan(t, endpoint, "route", map[string]any{"Role": "client",
		"ManifestDigest": liveHex(value.manifest), "StateRoot": "/run/state/route-network", "LocalRoleStateRoot": "/run/state/route-local-roles",
		"NetworkID": liveHex(value.network), "Authorities": []string{hex.EncodeToString(authority)},
		"Threshold": 1, "At": value.now.Format(time.RFC3339), "Seed": liveHex(value.plan.Seed),
		"Certificate": "/run/secure/cert.pem", "Key": "/run/secure/key.pem", "RawAttachment": true,
		"Stream": "/run/ardents/client-route/route.sock", "Deadline": "15s", "Lifetime": "15m"})
}
func writeBlockedServiceInputs(t *testing.T, root string, value liveFixture) [32]byte {
	t.Helper()
	now := value.now
	authorityPublic, authorityPrivate := deterministicLiveKey(0xb1)
	introductionPublic, introductionPrivate := deterministicLiveKey(0xb2)
	instancePublic, instancePrivate := deterministicLiveKey(0xb3)
	var instance [32]byte
	copy(instance[:], instancePublic)
	credential, err := (serviceconn.Credential{InstancePublic: instance, Generation: 1,
		NotBefore: now.Add(-time.Minute).Unix(), NotAfter: now.Add(30 * time.Minute).Unix(),
		NetworkID: value.network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		t.Fatal(err)
	}
	common := map[string]any{"NetworkID": liveHex(value.network),
		"AuthorityPublic":    hex.EncodeToString(authorityPublic),
		"IntroductionPublic": hex.EncodeToString(introductionPublic), "At": now.Format(time.RFC3339),
		"Deadline": "15s", "Lifetime": "15m", "PublicationFile": "/run/ardents/publication/current.bin",
		"CandidateView":      liveHex(sha256.Sum256([]byte("blocked-entry-view"))),
		"IsolationContext":   liveHex(sha256.Sum256([]byte("blocked-entry-isolation"))),
		"DestinationBinding": liveHex(sha256.Sum256([]byte("blocked-entry-destination"))),
		"RouteProfile":       "h3-route-tracer-v1", "WorkSafetyNotAfter": now.Add(25 * time.Minute).Unix(),
		"WorkSafetyMaximum": now.Add(25 * time.Minute).Unix(), "NoNewRecoveryAfter": now.Add(25 * time.Minute).Unix()}
	client := cloneLivePlan(common)
	client["Role"], client["BrokerID"] = "client", liveHex(sha256.Sum256([]byte("blocked-client-broker")))
	client["ConnectionPrincipal"] = liveHex(sha256.Sum256([]byte("blocked-client-principal")))
	client["Target"], client["ApplicationSocket"] = liveHex(credential.Target), "/run/ardents/client-app/app.sock"
	client["RouteSocket"], client["SendBytes"], client["ReceiveBytes"] = "/run/ardents/client-route/route.sock", 512, 64<<10
	publisher := cloneLivePlan(common)
	publisher["Role"], publisher["BrokerID"] = "publisher", liveHex(sha256.Sum256([]byte("blocked-publisher-broker")))
	publisher["ConnectionPrincipal"] = liveHex(sha256.Sum256([]byte("blocked-publisher-principal")))
	publisher["AdministrationPrincipal"] = liveHex(sha256.Sum256([]byte("blocked-administrator")))
	publisher["ApplicationSocket"], publisher["RouteSocket"] = "/run/ardents/publisher-app/app.sock", "/run/ardents/publisher-route/route.sock"
	publisher["AdministrationSocket"], publisher["IntroductionSocket"] = "/run/ardents/admin/admin.sock", "/run/ardents/introduction-ack/ack.sock"
	publisher["CredentialFile"], publisher["InstanceKeyFile"] = "/run/secure/credential.json", "/run/secure/instance.hex"
	publisher["GenerationStateFile"] = "/run/ardents/lifecycle/generation"
	publisher["SendBytes"], publisher["ReceiveBytes"] = 64<<10, 512
	clientDir, publisherDir := filepath.Join(root, "input", "client-service"), filepath.Join(root, "input", "publisher-service")
	writeLivePlan(t, clientDir, "plan", client)
	writeLivePlan(t, publisherDir, "plan", publisher)
	writeLivePlan(t, publisherDir, "credential", credential)
	writeLiveFile(t, filepath.Join(publisherDir, "instance.hex"), []byte(hex.EncodeToString(instancePrivate)))
	writeLiveFile(t, filepath.Join(publisherDir, "introduction-ack.hex"), []byte(hex.EncodeToString(introductionPrivate)))
	writeLiveFile(t, filepath.Join(root, "input", "client-app", "own.hex"), []byte(hex.EncodeToString(bytes.Repeat([]byte{17}, 32))))
	writeLiveFile(t, filepath.Join(root, "input", "client-app", "peer.hex"), []byte(hex.EncodeToString(bytes.Repeat([]byte{91}, 32))))
	writeLiveFile(t, filepath.Join(root, "input", "publisher-app", "own.hex"), []byte(hex.EncodeToString(bytes.Repeat([]byte{91}, 32))))
	writeLiveFile(t, filepath.Join(root, "input", "publisher-app", "peer.hex"), []byte(hex.EncodeToString(bytes.Repeat([]byte{17}, 32))))
	return credential.Target
}
func writeBlockedBridgeInputs(t *testing.T, root string, route liveFixture) {
	t.Helper()
	now := route.now
	network := prepareBlockedBridgeNetwork(t, filepath.Join(root, "bridge-network-source"), now)
	frontCertificate, frontKey, frontPin := writeBlockedFrontIdentity(t, now)
	envelope := blockedEnvelope([4]byte{203, 0, 113, 8}, 8480, frontPin)
	invite := blockedInvite(t, network, envelope, now)
	for _, role := range []string{"endpoint", "bridge"} {
		directory := filepath.Join(root, "input", role)
		copyTree(t, network.root, filepath.Join(directory, "bridge-network"))
		writeLiveFile(t, filepath.Join(directory, "invite.bin"), invite)
		rolesRoot := filepath.Join(directory, "local-roles")
		roles, err := localroles.Open(localroles.Config{Root: rolesRoot, Clock: time.Now, Create: true})
		if err != nil {
			t.Fatal(err)
		}
		if err := roles.Close(); err != nil {
			t.Fatal(err)
		}
		writeLivePlan(t, directory, "import", blockedImportPlan(network, role))
	}
	endpoint := filepath.Join(root, "input", "endpoint")
	writeLiveFile(t, filepath.Join(endpoint, "transition.bin"), blockedTransition(route.manifest))
	writeLivePlan(t, endpoint, "entry", map[string]any{"schema": "ardents-h3-bridge-entry-plan-v1",
		"bridge_state_root": "/run/state/bridge", "network_state_root": "/run/state/bridge-network",
		"network_id":          liveHex(network.snapshot.NetworkID),
		"network_authorities": []string{hex.EncodeToString(network.authorityPublic)}, "network_threshold": 1,
		"network_profile": "h3-role-probe-v1", "route_profile": "h3-route-tracer-v1",
		"local_role_state_root": "/run/state/local-roles", "binary": "/candidate/webtunnel-client",
		"candidate_state_root": "/run/state/candidate", "route_manifest_digest": liveHex(route.manifest),
		"transition_handle": "3", "time_confidence_file": "/run/secure/time-confidence"})
	bridge := filepath.Join(root, "input", "bridge")
	for _, role := range []string{"endpoint", "bridge"} {
		writeLiveFile(t, filepath.Join(root, "input", role, "time-confidence"), []byte("observed\n"))
	}
	writeLiveFile(t, filepath.Join(bridge, "front-cert.pem"), frontCertificate)
	writeLiveFile(t, filepath.Join(root, "input", "capacity-probe", "front-cert.pem"), frontCertificate)
	writeLiveFile(t, filepath.Join(root, "input", "capacity-probe", "corpus-seed.bin"),
		bytes.Repeat([]byte{0xc5}, 32))
	writeLiveFile(t, filepath.Join(root, "input", "probe", "corpus-seed.bin"),
		bytes.Repeat([]byte{0xc6}, 32))
	writeLiveFile(t, filepath.Join(bridge, "front-key.pem"), frontKey)
	writeNodeIdentity(t, filepath.Join(bridge, "identity-key.pem"), network.nodePrivate)
	probeCertificate, probeKey, probeRoot, probePin := blockedProbeCredentials(t, now)
	writeLiveFile(t, filepath.Join(bridge, "probe-cert.pem"), probeCertificate)
	writeLiveFile(t, filepath.Join(bridge, "probe-key.pem"), probeKey)
	writeLiveFile(t, filepath.Join(bridge, "probe-root.pem"), probeRoot)
	copyFile(t, filepath.Join(root, "input", "initiator", "plan.json"), filepath.Join(bridge, "initiator.json"))
	writeLivePlan(t, bridge, "serve", map[string]any{"schema": "ardents-h3-bridge-serve-plan-v1",
		"import_plan": "/run/secure/import.json", "binary": "/candidate/webtunnel-server",
		"candidate_state_root": "/run/state/candidate", "certificate": "/run/secure/front-cert.pem",
		"key": "/run/secure/front-key.pem", "next_initiator_plan": "/run/secure/initiator.json",
		"route_manifest_digest": liveHex(route.manifest), "deadline": now.Add(25 * time.Minute).Format(time.RFC3339),
		"identity_key": "/run/secure/identity-key.pem", "probe_listen": "127.0.0.1:4101",
		"probe_certificate": "/run/secure/probe-cert.pem", "probe_key": "/run/secure/probe-key.pem",
		"probe_client_root": "/run/secure/probe-root.pem", "probe_client_key_digests": []string{hex.EncodeToString(probePin[:])},
		"maximum_duty_ms": 1000, "drain_timeout_ms": 1000, "resource_profile": "h3-s-v1"})
	writeBlockedProbeInputs(t, root, frontCertificate, envelope, network.snapshot.Candidates[0].NodeID)
}
func prepareBlockedBridgeNetwork(t *testing.T, directory string, now time.Time) blockedBridgeNetwork {
	t.Helper()
	networkID := sha256.Sum256([]byte("stage-5-blocked-bridge-network"))
	authority := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0xa9}, ed25519.SeedSize))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, ed25519.SeedSize))
	record := blockedNodeRecord(networkID, node, now)
	epoch, digest := blockedNodeEpoch(networkID, authority, record, now)
	material := blockedMaterialization(digest, record)
	public := authority.Public().(ed25519.PublicKey)
	owner, err := state.Open(state.Config{Root: directory, NetworkID: networkID,
		Authorities: map[[32]byte]ed25519.PublicKey{sha256.Sum256(public): public}, Threshold: 1, Now: now})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, acceptErr := owner.Accept(context.Background(), epoch, [][]byte{record}, [][]byte{material})
	if closeErr := owner.Close(); acceptErr == nil {
		acceptErr = closeErr
	}
	if acceptErr != nil {
		t.Fatal(acceptErr)
	}
	return blockedBridgeNetwork{root: directory, snapshot: snapshot, authorityPublic: public,
		nodePrivate: node, domainProof: material}
}

func blockedNodeRecord(network [32]byte, private ed25519.PrivateKey, now time.Time) []byte {
	var raw bytes.Buffer
	raw.WriteString("ARNR")
	raw.WriteByte(1)
	raw.Write(network[:])
	nodeID := sha256.Sum256([]byte("stage-5-blocked-bridge-node"))
	raw.Write(nodeID[:])
	writeU64(&raw, 1)
	writeI64(&raw, now.Add(-time.Hour).Unix())
	writeI64(&raw, now.Add(time.Hour).Unix())
	writeText(&raw, "stage-5-blocked-bridge-family")
	raw.WriteByte(1)
	writeText(&raw, "127.0.0.1:4101")
	writeU16(&raw, 4)
	raw.Write(private.Public().(ed25519.PublicKey))
	raw.Write(ed25519.Sign(private, raw.Bytes()))
	return raw.Bytes()
}

func blockedNodeEpoch(network [32]byte, authority ed25519.PrivateKey, record []byte, now time.Time) ([]byte, [32]byte) {
	var raw bytes.Buffer
	raw.WriteString("AREP")
	raw.WriteByte(1)
	raw.Write(network[:])
	writeU64(&raw, 1)
	raw.Write(make([]byte, 32))
	writeI64(&raw, now.Add(-time.Minute).Unix())
	writeI64(&raw, now.Add(time.Hour).Unix())
	writeU32(&raw, 1)
	writeText(&raw, "h3-role-probe-v1")
	leaf := blockedMerkleLeaf(record)
	raw.Write(leaf[:])
	raw.Write(leaf[:])
	writeU32(&raw, 1)
	empty := sha256.Sum256([]byte{0x12})
	raw.Write(empty[:])
	writeU32(&raw, 0)
	assignmentSeed := sha256.Sum256([]byte("stage-5-blocked-assignment"))
	raw.Write(assignmentSeed[:])
	writeText(&raw, "ardents-h3-role-domain-v1")
	writeU32(&raw, 1)
	writeU32(&raw, 4)
	writeU16(&raw, 1)
	writeU16(&raw, 1)
	writeU32(&raw, 4)
	raw.WriteByte(1)
	writeText(&raw, "initiator")
	writeU16(&raw, 1)
	writeU32(&raw, 4)
	digest := sha256.Sum256(raw.Bytes())
	public := authority.Public().(ed25519.PublicKey)
	raw.WriteByte(1)
	authorityID := sha256.Sum256(public)
	raw.Write(authorityID[:])
	raw.Write(ed25519.Sign(authority, digest[:]))
	return raw.Bytes(), digest
}

func blockedMaterialization(digest [32]byte, record []byte) []byte {
	var raw bytes.Buffer
	raw.Write(digest[:])
	writeU32(&raw, 0)
	writeU32(&raw, uint32(len(record)))
	raw.Write(record)
	writeU16(&raw, 0)
	return raw.Bytes()
}

func blockedMerkleLeaf(record []byte) [32]byte {
	raw := make([]byte, 5+len(record))
	binary.BigEndian.PutUint32(raw[1:5], uint32(len(record)))
	copy(raw[5:], record)
	return sha256.Sum256(raw)
}

func blockedInvite(t *testing.T, network blockedBridgeNetwork, candidate []byte, now time.Time) []byte {
	t.Helper()
	return blockedInviteForSlot(t, network, candidate, now, 0)
}

func blockedInviteForSlot(t *testing.T, network blockedBridgeNetwork, candidate []byte, now time.Time, slot byte) []byte {
	t.Helper()
	facts, ok := network.snapshot.BridgeCandidateByKey(network.snapshot.Candidates[0].KeyID)
	if !ok {
		t.Fatal("blocked Bridge candidate is absent")
	}
	var body bytes.Buffer
	writeU16(&body, 1)
	body.Write(network.snapshot.NetworkID[:])
	writeU64(&body, network.snapshot.Epoch)
	body.Write(network.snapshot.Digest[:])
	writeBytes(&body, []byte("h3-route-tracer-v1"), 1)
	body.WriteByte(1)
	body.Write(facts.NodeID[:])
	body.Write(facts.FamilyID[:])
	body.Write(facts.RecordDigest[:])
	writeBytes(&body, network.domainProof, 2)
	writeI64(&body, facts.AssignmentNotAfter.Unix())
	writeI64(&body, now.Add(-time.Minute).Unix())
	writeI64(&body, now.Add(25*time.Minute).Unix())
	body.Write([]byte{1, slot, 0})
	writeBytes(&body, candidate, 2)
	body.Write(facts.KeyID[:])
	var raw bytes.Buffer
	raw.WriteString("ardents-h3-bi1")
	writeU16(&raw, uint16(body.Len()))
	raw.Write(body.Bytes())
	raw.Write(ed25519.Sign(network.nodePrivate,
		append([]byte("ardents-h3-bridge-invite-signature-v1\x00"), body.Bytes()...)))
	return raw.Bytes()
}

func blockedEnvelope(address [4]byte, port uint16, pin [32]byte) []byte {
	var raw bytes.Buffer
	raw.WriteString("ardents-h3-wt1")
	raw.WriteByte(1)
	writeBytes(&raw, []byte("webtunnel-v0.0.6"), 1)
	raw.Write(address[:])
	writeU16(&raw, port)
	writeBytes(&raw, []byte("/entry"), 2)
	writeBytes(&raw, []byte("front.example"), 1)
	raw.Write(pin[:])
	return raw.Bytes()
}

func blockedTransition(manifest [32]byte) []byte {
	var raw bytes.Buffer
	raw.WriteString("ardents-h3-bridge-transition-v1")
	raw.Write(bytes.Repeat([]byte{1}, 32))
	raw.WriteByte(1)
	raw.Write(bytes.Repeat([]byte{2}, 32))
	writeU64(&raw, 1)
	raw.Write(manifest[:])
	return raw.Bytes()
}

func writeBlockedFrontIdentity(t *testing.T, now time.Time) ([]byte, []byte, [32]byte) {
	return issueBlockedCertificate(t, now, "front.example", []string{"front.example"}, x509.ExtKeyUsageServerAuth, nil)
}

func blockedProbeCredentials(t *testing.T, now time.Time) ([]byte, []byte, []byte, [32]byte) {
	t.Helper()
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{SerialNumber: big.NewInt(100), Subject: pkix.Name{CommonName: "blocked-probe-root"},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, caPublic, caPrivate)
	if err != nil {
		t.Fatal(err)
	}
	serverCert, serverKey, _ := issueBlockedCertificate(t, now, "probe.bridge", nil, x509.ExtKeyUsageServerAuth,
		&certificateIssuer{certificate: ca, der: caDER, private: caPrivate})
	clientPublic, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := x509.MarshalPKIXPublicKey(clientPublic)
	if err != nil {
		t.Fatal(err)
	}
	return serverCert, serverKey, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}), sha256.Sum256(encoded)
}

type certificateIssuer struct {
	certificate *x509.Certificate
	der         []byte
	private     ed25519.PrivateKey
}

func issueBlockedCertificate(t *testing.T, now time.Time, commonName string, names []string,
	usage x509.ExtKeyUsage, issuer *certificateIssuer) ([]byte, []byte, [32]byte) {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(now.UnixNano()), Subject: pkix.Name{CommonName: commonName},
		DNSNames: names, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage}}
	parent, signer := template, private
	if issuer != nil {
		parent, signer = issuer.certificate, issuer.private
	}
	der, err := x509.CreateCertificate(rand.Reader, template, parent, public, signer)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	if issuer != nil {
		certificate = append(certificate, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: issuer.der})...)
	}
	return certificate, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}), sha256.Sum256(der)
}

func copyLiveIdentity(t *testing.T, fixture liveFixture, index int, destination string) {
	t.Helper()
	copyFile(t, filepath.Join(fixture.root, "secrets", filepath.Base(fixture.identities[index].cert)), filepath.Join(destination, "cert.pem"))
	copyFile(t, filepath.Join(fixture.root, "secrets", filepath.Base(fixture.identities[index].key)), filepath.Join(destination, "key.pem"))
}

func deterministicLiveKeyBytes(marker byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{marker}, ed25519.SeedSize))
}

func writeNodeIdentity(t *testing.T, path string, private ed25519.PrivateKey) {
	t.Helper()
	raw, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	writeLiveFile(t, path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: raw}))
}
