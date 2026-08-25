package service_test

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

func TestReferenceC2RunsEveryRoleInSeparateProcesses(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	deadline := now.Add(20 * time.Second)
	network, digest := referenceC2ID(1), referenceC2ID(2)
	introductionID, rendezvousID := referenceC2ID(3), referenceC2ID(4)
	responderID, initiatorID := referenceC2ID(5), referenceC2ID(6)
	introductionMaterial := referenceC2Certificate(t, 3, "introduction")
	rendezvousMaterial := referenceC2Certificate(t, 4, "rendezvous")
	responderMaterial := referenceC2Certificate(t, 5, "responder")
	initiatorMaterial := referenceC2Certificate(t, 6, "initiator")
	gatewayMaterial := referenceC2Certificate(t, 7, "gateway")
	introductionAddress, rendezvousAddress := referenceC2Address(t), referenceC2Address(t)
	responderAddress, initiatorAddress, gatewayAddress := referenceC2Address(t), referenceC2Address(t), referenceC2Address(t)
	join, reachability := referenceC2ID(7), referenceC2ID(8)
	slotAttachment, serviceAttachment, resolutionAttachment := referenceC2ID(9), referenceC2ID(10), referenceC2ID(12)
	transitAuthorityPublic, transitAuthorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	slotCredential := referenceC2TransitCredential(t, transitAuthorityPrivate, network, digest, 10, introductionID, route.IntroductionRole, slotAttachment, deadline, 31)
	responderCredential := referenceC2TransitCredential(t, transitAuthorityPrivate, network, digest, 10, responderID, route.ResponderRole, serviceAttachment, deadline, 32)
	introductionCredential := referenceC2TransitCredential(t, transitAuthorityPrivate, network, digest, 10, introductionID, route.IntroductionRole, serviceAttachment, deadline, 33)
	invite := "entry-invite"
	inviteID := referenceC2ID(11)

	root := t.TempDir()
	publicationPath := filepath.Join(root, "publication.json")
	configPath := filepath.Join(root, "reference-c2.json")
	readyRoot := filepath.Join(root, "ready")
	if err := os.Mkdir(readyRoot, 0o700); err != nil {
		t.Fatal(err)
	}
	completePath := filepath.Join(root, "complete")
	fixture := map[string]any{
		"Schema": "ardents-e2e-reference-c2-v1", "Network": referenceC2Hex(network), "Digest": referenceC2Hex(digest),
		"Epoch": 10, "Deadline": deadline.Format(time.RFC3339), "PublicationPath": publicationPath, "PublisherRoot": filepath.Join(root, "publisher-state"),
		"GatewayRoot": filepath.Join(root, "gateway-state"), "GatewayProfilePath": filepath.Join(root, "gateway-profile.json"),
		"ReadyRoot": readyRoot, "CompletePath": completePath,
		"Introduction": referenceC2Peer(introductionID, introductionMaterial, introductionAddress), "Rendezvous": referenceC2Peer(rendezvousID, rendezvousMaterial, rendezvousAddress),
		"Responder": referenceC2Peer(responderID, responderMaterial, responderAddress), "Initiator": referenceC2Peer(initiatorID, initiatorMaterial, initiatorAddress),
		"Gateway":    referenceC2Peer(referenceC2ID(13), gatewayMaterial, gatewayAddress),
		"JoinHandle": referenceC2Hex(join), "Reachability": referenceC2Hex(reachability), "SlotAttachment": referenceC2Hex(slotAttachment),
		"ServiceAttachment": referenceC2Hex(serviceAttachment), "ResolutionAttachment": referenceC2Hex(resolutionAttachment),
		"TransitAuthority": hex.EncodeToString(transitAuthorityPublic), "SlotCredential": slotCredential, "ResponderCredential": responderCredential,
		"IntroductionCredential": introductionCredential, "InviteID": referenceC2Hex(inviteID), "Invite": invite,
	}
	raw, err := json.Marshal(fixture)
	if err != nil || os.WriteFile(configPath, raw, 0o600) != nil {
		t.Fatal("write process C2 fixture configuration")
	}
	binary := buildE2EFixtureCommand(t, "reference-c2")
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	transit := make(map[string]<-chan commandResult, 4)
	for _, role := range []string{"rendezvous", "initiator", "introduction", "responder"} {
		transit[role] = startCommand(ctx, root, binary, role, configPath)
		if err := referenceC2WaitForFile(ctx, filepath.Join(readyRoot, role)); err != nil {
			process := <-transit[role]
			t.Fatalf("C2 transit process %s did not become ready: %v\n%s", role, err, process.output)
		}
	}
	gateway := startCommand(ctx, root, binary, "gateway", configPath)
	publisher := startCommand(ctx, root, binary, "publisher", configPath)
	if err := referenceC2WaitForFile(ctx, publicationPath); err != nil {
		process := <-publisher
		t.Fatalf("C2 Publisher process did not publish: %v\n%s", err, process.output)
	}
	if err := referenceC2WaitForFile(ctx, filepath.Join(readyRoot, "gateway")); err != nil {
		process := <-gateway
		t.Fatalf("C2 Gateway process did not become ready: %v\n%s", err, process.output)
	}
	user := startCommand(ctx, root, binary, "user", configPath)
	processes := map[string]commandResult{"user": <-user, "publisher": <-publisher}
	if err := os.WriteFile(completePath, []byte("complete\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for role, process := range transit {
		processes[role] = <-process
	}
	processes["gateway"] = <-gateway
	for role, process := range processes {
		if process.err != nil {
			t.Fatalf("C2 %s Endpoint process failed: %v\n%s", role, process.err, process.output)
		}
		var observed struct {
			Schema, Role, Class string
			Passed              bool
		}
		line := strings.TrimSpace(string(process.output))
		if index := strings.LastIndex(line, "\n"); index >= 0 {
			line = line[index+1:]
		}
		if err := json.Unmarshal([]byte(line), &observed); err != nil || observed.Schema != "ardents-e2e-reference-c2-result-v1" || observed.Role != role || !observed.Passed || observed.Class == "" {
			t.Fatalf("C2 Endpoint process result = %q / %+v / %v", process.output, observed, err)
		}
		if role == "rendezvous" || role == "initiator" || role == "introduction" || role == "responder" || role == "gateway" {
			if observed.Class != "drained" {
				t.Fatalf("C2 transit process %s result class = %q, want drained", role, observed.Class)
			}
		}
	}
}

func referenceC2WaitForFile(ctx context.Context, path string) error {
	ticker := time.NewTicker(20 * time.Millisecond)
	defer ticker.Stop()
	for {
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

type referenceC2CertificateMaterial struct {
	public      [32]byte
	certificate string
	privateKey  string
}

func referenceC2Certificate(t *testing.T, serial int64, name string) referenceC2CertificateMaterial {
	t.Helper()
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: name}, NotBefore: time.Now().Add(-time.Minute),
		NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		t.Fatal(err)
	}
	privateDER, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		t.Fatal(err)
	}
	var material referenceC2CertificateMaterial
	copy(material.public[:], public)
	material.certificate = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	material.privateKey = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateDER}))
	return material
}

func referenceC2Address(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	_ = listener.Close()
	return address
}

func referenceC2Peer(nodeID [32]byte, material referenceC2CertificateMaterial, endpoint string) map[string]string {
	return map[string]string{"NodeID": referenceC2Hex(nodeID), "PublicKey": referenceC2Hex(material.public), "Endpoint": endpoint,
		"Certificate": material.certificate, "PrivateKey": material.privateKey}
}

func referenceC2TransitCredential(t *testing.T, authority ed25519.PrivateKey, network, digest [32]byte, epoch uint64, transitNode [32]byte,
	role byte, attachment [32]byte, deadline time.Time, marker byte) map[string]string {
	t.Helper()
	material := referenceC2Certificate(t, int64(marker), "transit-client")
	certificate, err := tls.X509KeyPair([]byte(material.certificate), []byte(material.privateKey))
	if err != nil || len(certificate.Certificate) != 1 {
		t.Fatal("create transit client certificate")
	}
	certificate.Leaf, err = x509.ParseCertificate(certificate.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	digestKey, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		t.Fatal(err)
	}
	grant, err := route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(authority.Public().(ed25519.PublicKey)), GrantID: referenceC2ID(marker),
		NetworkID: network, Digest: digest, AttachmentID: attachment, TransitNodeID: transitNode, ClientKeyDigest: digestKey,
		Epoch: epoch, TransitRole: role, NotAfter: deadline}, authority)
	if err != nil {
		t.Fatal(err)
	}
	return map[string]string{"Grant": base64.RawStdEncoding.EncodeToString(grant), "Certificate": material.certificate, "PrivateKey": material.privateKey}
}

func referenceC2Hex(value [32]byte) string { return hex.EncodeToString(value[:]) }

func referenceC2ID(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}
