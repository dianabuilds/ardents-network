package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/application/broker"
	endpointapi "github.com/dianabuilds/ardents-network/internal/endpoint"
	"github.com/dianabuilds/ardents-network/internal/entry"
	"github.com/dianabuilds/ardents-network/internal/service/publication"
	"github.com/dianabuilds/ardents-network/internal/service/targetlink"
)

type peer struct {
	NodeID, PublicKey, Endpoint string
}

type config struct {
	Schema, Network, Digest, Deadline, PublicationPath, PublisherRoot string
	Epoch                                                             uint64
	Introduction, Rendezvous, Responder, Initiator                    peer
	JoinHandle, Reachability, SlotAttachment, ServiceAttachment       string
	SlotAuthorization, ResponderAuthorization, InviteID, Invite       string
}

type publicationEnvelope struct {
	AuthorityPublic, Publication, TargetLink string
}

type result struct {
	Schema, Role, Class string
	Passed              bool
}

func main() {
	if len(os.Args) != 3 || (os.Args[1] != "publisher" && os.Args[1] != "user") {
		fmt.Fprintln(os.Stderr, "usage: reference-c2 (publisher|user) CONFIG")
		os.Exit(2)
	}
	input, err := readConfig(os.Args[2])
	if err == nil && os.Args[1] == "publisher" {
		err = runPublisher(input)
	} else if err == nil {
		err = runUser(input)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

func readConfig(path string) (config, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 16<<10 {
		return config{}, errors.New("C2 fixture configuration is unavailable")
	}
	var input config
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		return config{}, err
	}
	if input.Schema != "ardents-e2e-reference-c2-v1" || input.Epoch == 0 || input.PublicationPath == "" || input.PublisherRoot == "" ||
		input.SlotAuthorization == "" || input.ResponderAuthorization == "" || input.Invite == "" {
		return config{}, errors.New("C2 fixture configuration is incomplete")
	}
	if _, err := input.deadline(); err != nil {
		return config{}, err
	}
	for _, value := range []string{input.Network, input.Digest, input.JoinHandle, input.Reachability, input.SlotAttachment, input.ServiceAttachment, input.InviteID} {
		if _, err := fixed(value); err != nil {
			return config{}, err
		}
	}
	for _, value := range []peer{input.Introduction, input.Rendezvous, input.Responder, input.Initiator} {
		if _, err := value.decode(); err != nil {
			return config{}, err
		}
	}
	return input, nil
}

func (input config) deadline() (time.Time, error) {
	deadline, err := time.Parse(time.RFC3339, input.Deadline)
	if err != nil || !time.Now().UTC().Before(deadline) {
		return time.Time{}, errors.New("C2 fixture deadline is invalid")
	}
	return deadline.UTC(), nil
}

func (value peer) decode() (endpointapi.TransitPeer, error) {
	nodeID, err := fixed(value.NodeID)
	if err != nil {
		return endpointapi.TransitPeer{}, err
	}
	public, err := fixed(value.PublicKey)
	if err != nil || value.Endpoint == "" {
		return endpointapi.TransitPeer{}, errors.New("C2 fixture peer is invalid")
	}
	return endpointapi.TransitPeer{NodeID: nodeID, PublicKey: public, Endpoint: value.Endpoint}, nil
}

func runPublisher(input config) error {
	deadline, _ := input.deadline()
	now := time.Now().UTC().Truncate(time.Second)
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	introduction, _ := input.Introduction.decode()
	rendezvous, _ := input.Rendezvous.decode()
	responder, _ := input.Responder.decode()
	join, _ := fixed(input.JoinHandle)
	reachability, _ := fixed(input.Reachability)
	slotAttachment, _ := fixed(input.SlotAttachment)
	authorityPublic, authorityPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	introductionPublic, introductionPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	instancePublic, instancePrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	hpkePrivate, err := hpkePrivateKey()
	if err != nil {
		return err
	}
	var authority, instance, hpkePublic [32]byte
	copy(authority[:], authorityPublic)
	copy(instance[:], instancePublic)
	copy(hpkePublic[:], hpkePrivate.PublicKey().Bytes())
	credential, err := (publication.Credential{AuthorityPublic: authority, InstancePublic: instance, IntroductionHPKEPublic: hpkePublic,
		Generation: 1, NotBefore: now.Add(-time.Second).Unix(), NotAfter: deadline.Unix(), NetworkID: network, Capabilities: 3}).Issue(authorityPrivate)
	if err != nil {
		return err
	}
	brokerID, principal, administrator := identifier(41), identifier(42), identifier(43)
	publisher, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: brokerID, AuthorityPublic: authorityPublic,
		IntroductionPublic: introductionPublic, ConnectionPrincipal: principal, AdministrationPrincipal: administrator, PublicationRoot: input.PublisherRoot})
	if err != nil {
		return err
	}
	defer publisher.Close()
	administration, err := publisher.Admit(administrator, broker.Administration)
	if err != nil {
		return err
	}
	published, err := publisher.Publish(context.Background(), endpointapi.PublicationRequest{Principal: administrator, Capability: administration,
		Credential: credential, InstancePrivate: instancePrivate, IntroductionAcknowledgement: acknowledgement(credential, introductionPrivate, brokerID), At: now})
	if err != nil {
		return err
	}
	link, err := targetlink.Encode(targetlink.Link{Network: network, Target: credential.Target})
	if err != nil {
		return err
	}
	if err := writePublication(input.PublicationPath, publicationEnvelope{AuthorityPublic: hex.EncodeToString(authorityPublic),
		Publication: base64.RawStdEncoding.EncodeToString(published.Record), TargetLink: link}); err != nil {
		return err
	}
	slot, err := publisher.OpenPublisherIntroduction(context.Background(), endpointapi.PublisherIntroductionRequest{
		Profile: endpointapi.PublisherIntroductionProfile{NetworkID: network, Digest: digest, Epoch: input.Epoch, Introduction: introduction,
			Rendezvous: rendezvous, Responder: responder, SlotAttachmentID: slotAttachment, Reachability: reachability, JoinHandle: join,
			NotAfter: deadline, SlotAuthorization: []byte(input.SlotAuthorization), ResponderAuthorization: []byte(input.ResponderAuthorization)},
		HPKEPrivate: hpkePrivate, At: now})
	if err != nil {
		return err
	}
	defer slot.Close()
	capability, err := publisher.Admit(principal, broker.Connection)
	if err != nil {
		return err
	}
	application, service := net.Pipe()
	defer service.Close()
	go serveStatic(service)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	outcome, err := slot.Accept(ctx, endpointapi.InboundConnectionRequest{Principal: principal, Capability: capability,
		Application: application, BytesEachDirection: 64 << 10, At: now})
	if outcome.Class == "" {
		return errors.Join(err, errors.New("Publisher C2 fixture did not complete a classified Service Connection"))
	}
	return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "publisher", Class: outcome.Class, Passed: true})
}

func runUser(input config) error {
	deadline, _ := input.deadline()
	envelope, err := readPublication(input.PublicationPath)
	if err != nil {
		return err
	}
	network, _ := fixed(input.Network)
	digest, _ := fixed(input.Digest)
	authority, err := hex.DecodeString(envelope.AuthorityPublic)
	if err != nil || len(authority) != ed25519.PublicKeySize {
		return errors.New("Publisher authority is invalid")
	}
	publicationRecord, err := base64.RawStdEncoding.DecodeString(envelope.Publication)
	if err != nil || len(publicationRecord) == 0 {
		return errors.New("Publisher publication is invalid")
	}
	introduction, _ := input.Introduction.decode()
	rendezvous, _ := input.Rendezvous.decode()
	initiator, _ := input.Initiator.decode()
	join, _ := fixed(input.JoinHandle)
	reachability, _ := fixed(input.Reachability)
	serviceAttachment, _ := fixed(input.ServiceAttachment)
	inviteID, _ := fixed(input.InviteID)
	userPrincipal := identifier(42)
	user, err := endpointapi.New(endpointapi.Setup{NetworkID: network, BrokerID: identifier(44), AuthorityPublic: authority,
		IntroductionPublic: make([]byte, ed25519.PublicKeySize), ConnectionPrincipal: userPrincipal})
	if err != nil {
		return err
	}
	defer user.Close()
	capability, err := user.Admit(userPrincipal, broker.Connection)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Truncate(time.Second)
	site, err := user.OpenUserReferenceSite(context.Background(), endpointapi.UserReferenceSiteRequest{
		Introduction: endpointapi.UserIntroductionRouteRequest{TargetLink: envelope.TargetLink, Publication: publicationRecord,
			Introduction: endpointapi.UserIntroductionProfile{NetworkID: network, Digest: digest, Epoch: input.Epoch, Introduction: introduction,
				RendezvousNodeID: rendezvous.NodeID, Reachability: reachability, JoinHandle: join, NotAfter: deadline, SubmissionAuthorization: join[:]},
			Entry: entryAcquirer{candidate: entry.Candidate{NodeID: initiator.NodeID, PublicKey: initiator.PublicKey, Endpoint: initiator.Endpoint},
				presentation: entry.Presentation{InviteID: inviteID, Invite: []byte(input.Invite)}}, Initiator: initiator, Rendezvous: rendezvous,
			AttachmentID: serviceAttachment, EndpointHandshake: identifier(45), At: now},
		Routes: map[string]string{"": "/"}, Principal: userPrincipal, Capability: capability, BytesEachDirection: 64 << 10})
	if err != nil {
		return err
	}
	defer site.Close()
	var ready endpointapi.ReferenceReady
	select {
	case ready = <-site.Ready():
		if ready.URL == "" {
			return errors.New("User C2 fixture did not receive Reference Site readiness")
		}
	case <-time.After(time.Until(deadline)):
		return errors.New("User C2 fixture timed out before Reference Site readiness")
	}
	response, err := (&http.Client{Transport: &http.Transport{Proxy: nil}}).Get(ready.URL)
	if err != nil {
		return err
	}
	body, readErr := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if readErr != nil || response.StatusCode != http.StatusOK || string(body) != "<h1>Reference</h1>" {
		return errors.New("User C2 fixture did not receive the Reference Site response")
	}
	if err := site.Close(); err != nil {
		return err
	}
	select {
	case outcome := <-site.Done():
		if outcome.Result.Class == "" {
			return errors.New("User C2 fixture did not receive a classified Service Connection result")
		}
		return json.NewEncoder(os.Stdout).Encode(result{Schema: "ardents-e2e-reference-c2-result-v1", Role: "user", Class: outcome.Result.Class, Passed: true})
	case <-time.After(time.Until(deadline)):
		return errors.New("User C2 fixture did not receive a terminal result")
	}
}

type entryAcquirer struct {
	candidate    entry.Candidate
	presentation entry.Presentation
}

func (input entryAcquirer) Acquire(ctx context.Context, attempt entry.Attempt, opener entry.CandidateOpener) (net.Conn, func() error, error) {
	connection, cleanup, complete, err := opener(ctx, input.candidate, input.presentation, attempt.Deadline)
	if err != nil || !complete {
		return nil, cleanup, err
	}
	return connection, cleanup, nil
}

func serveStatic(connection net.Conn) {
	defer connection.Close()
	request, err := http.ReadRequest(bufio.NewReader(connection))
	if err != nil || request.Method != http.MethodGet || request.URL.Path != "/" || request.Host != "reference" {
		return
	}
	_, _ = io.WriteString(connection, "HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: 18\r\nConnection: close\r\n\r\n<h1>Reference</h1>")
}

func writePublication(path string, value publicationEnvelope) error {
	if filepath.Dir(path) == "." {
		return errors.New("Publisher publication path is not absolute")
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temporary, path)
}

func readPublication(path string) (publicationEnvelope, error) {
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) == 0 || len(raw) > 8<<10 {
		return publicationEnvelope{}, errors.New("Publisher publication is unavailable")
	}
	var value publicationEnvelope
	if err := json.Unmarshal(raw, &value); err != nil || value.AuthorityPublic == "" || value.Publication == "" || value.TargetLink == "" {
		return publicationEnvelope{}, errors.New("Publisher publication is invalid")
	}
	return value, nil
}

func acknowledgement(credential publication.Credential, private ed25519.PrivateKey, brokerID [32]byte) []byte {
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

func hpkePrivateKey() (hpke.PrivateKey, error) {
	private, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	return hpke.NewDHKEMPrivateKey(private)
}

func fixed(encoded string) ([32]byte, error) {
	var value [32]byte
	raw, err := hex.DecodeString(encoded)
	if err != nil || len(raw) != len(value) {
		return value, errors.New("C2 fixture fixed value is invalid")
	}
	copy(value[:], raw)
	return value, nil
}

func identifier(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value + byte(index)
	}
	return result
}
