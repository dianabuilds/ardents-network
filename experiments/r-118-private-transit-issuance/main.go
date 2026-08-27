//go:build ignore

// R-118 disposable private Transit Grant issuance data-flow experiment.
package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/binary"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"os"
	"time"

	"github.com/dianabuilds/ardents-network/internal/route"
)

const requestPrefix = "ardents-r118-transit-issuance-request-v1\x00"

type issuanceRequest struct {
	Network, Digest, TransitNode, Attachment, ClientKey [32]byte
	Epoch                                               uint64
	Role                                                byte
	NotAfter                                            time.Time
}

type result struct {
	Schema               string   `json:"schema"`
	Case                 string   `json:"case"`
	Passed               bool     `json:"passed"`
	IssuerFields         []string `json:"issuer_fields"`
	TargetInRequest      bool     `json:"target_in_request"`
	ServiceNameInRequest bool     `json:"service_name_in_request"`
	Outcome              string   `json:"outcome"`
}

func main() {
	caseName := flag.String("case", "exact", "exact|target|node-substitution|expiry-substitution|wrong-key|replay")
	flag.Parse()
	result, err := run(*caseName)
	if result.Schema != "" {
		if encodeErr := json.NewEncoder(os.Stdout).Encode(result); encodeErr != nil {
			fmt.Fprintln(os.Stderr, encodeErr)
			os.Exit(1)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "failure="+err.Error())
		os.Exit(1)
	}
}

func run(caseName string) (result, error) {
	if caseName != "exact" && caseName != "target" && caseName != "node-substitution" && caseName != "expiry-substitution" && caseName != "wrong-key" && caseName != "replay" {
		return result{}, errors.New("experiment case is invalid")
	}
	now := time.Now().UTC().Truncate(time.Second)
	network, digest, introduction, attachment := marker(1), marker(2), marker(3), marker(4)
	certificate, err := clientCertificate()
	if err != nil {
		return result{}, err
	}
	key, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil {
		return result{}, err
	}
	stateDuty := issuanceRequest{Network: network, Digest: digest, TransitNode: introduction, Epoch: 7,
		Role: route.IntroductionRole, NotAfter: now.Add(time.Minute)}
	request := issuanceRequest{Network: network, Digest: digest, TransitNode: introduction, Attachment: attachment,
		ClientKey: key, Epoch: 7, Role: route.IntroductionRole, NotAfter: stateDuty.NotAfter}
	if caseName == "node-substitution" {
		request.TransitNode = marker(5)
	}
	if caseName == "expiry-substitution" {
		request.NotAfter = request.NotAfter.Add(time.Minute)
	}
	raw, err := encodeRequest(request)
	if err != nil {
		return result{}, err
	}
	target, serviceName := marker(231), []byte("reference.ard")
	if caseName == "target" {
		raw = append(raw, target[:]...)
	}
	result := result{Schema: "ardents-r118-private-transit-issuance-result-v1", Case: caseName,
		IssuerFields:    []string{"network_id", "state_digest", "epoch", "transit_node_id", "transit_role", "attachment_id", "client_key_digest", "not_after"},
		TargetInRequest: bytes.Contains(raw, target[:]), ServiceNameInRequest: bytes.Contains(raw, serviceName)}
	issuerPublic, issuerPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return result, err
	}
	grant, issueErr := issue(issuerPrivate, raw, stateDuty)
	if caseName == "target" || caseName == "node-substitution" || caseName == "expiry-substitution" {
		if issueErr == nil {
			return result, errors.New("issuer accepted an invalid issuance request")
		}
		result.Passed, result.Outcome = true, "issuer-refused"
		return result, nil
	}
	if issueErr != nil {
		return result, issueErr
	}
	certificateForAdmission := certificate
	if caseName == "wrong-key" {
		certificateForAdmission, err = clientCertificate()
		if err != nil {
			return result, err
		}
	}
	spent := map[[32]byte]bool{}
	admitErr := admit(issuerPublic, grant, request, certificateForAdmission, now, spent)
	if caseName == "wrong-key" {
		if admitErr == nil {
			return result, errors.New("node accepted a substituted local TLS key")
		}
		result.Passed, result.Outcome = true, "node-refused-key-substitution"
		return result, nil
	}
	if admitErr != nil {
		return result, admitErr
	}
	if caseName == "replay" {
		if replayErr := admit(issuerPublic, grant, request, certificate, now, spent); replayErr == nil {
			return result, errors.New("node accepted a replayed Transit Grant")
		}
		result.Passed, result.Outcome = true, "node-refused-replay"
		return result, nil
	}
	result.Passed, result.Outcome = true, "issued-and-admitted"
	return result, nil
}

func issue(authority ed25519.PrivateKey, raw []byte, expected issuanceRequest) ([]byte, error) {
	request, err := decodeRequest(raw)
	if err != nil || request.Network != expected.Network || request.Digest != expected.Digest || request.TransitNode != expected.TransitNode ||
		request.Epoch != expected.Epoch || request.Role != expected.Role || !request.NotAfter.Equal(expected.NotAfter) {
		return nil, errors.New("issuer request is not the current State transit duty")
	}
	grantID, err := randomID()
	if err != nil {
		return nil, err
	}
	return route.IssueTransitGrant(route.TransitGrant{IssuerID: sha256.Sum256(authority.Public().(ed25519.PublicKey)), GrantID: grantID,
		NetworkID: request.Network, Digest: request.Digest, AttachmentID: request.Attachment, TransitNodeID: request.TransitNode,
		ClientKeyDigest: request.ClientKey, Epoch: request.Epoch, TransitRole: request.Role, NotAfter: request.NotAfter}, authority)
}

func admit(authority ed25519.PublicKey, raw []byte, request issuanceRequest, certificate tls.Certificate, now time.Time, spent map[[32]byte]bool) error {
	grant, err := route.VerifyTransitGrant(raw, authority)
	if err != nil {
		return err
	}
	key, err := route.ClientTLSKeyDigest(certificate.Leaf)
	if err != nil || grant.NetworkID != request.Network || grant.Digest != request.Digest || grant.AttachmentID != request.Attachment ||
		grant.TransitNodeID != request.TransitNode || grant.ClientKeyDigest != key || grant.Epoch != request.Epoch ||
		grant.TransitRole != request.Role || !now.Before(grant.NotAfter) || spent[grant.GrantID] {
		return errors.New("node admission does not match the exact issued transit grant")
	}
	spent[grant.GrantID] = true
	return nil
}

func encodeRequest(value issuanceRequest) ([]byte, error) {
	if value.Network == [32]byte{} || value.Digest == [32]byte{} || value.TransitNode == [32]byte{} || value.Attachment == [32]byte{} ||
		value.ClientKey == [32]byte{} || value.Epoch == 0 || value.Role != route.IntroductionRole || value.NotAfter.IsZero() ||
		!value.NotAfter.Equal(value.NotAfter.UTC().Truncate(time.Second)) {
		return nil, errors.New("issuance request is invalid")
	}
	raw := make([]byte, 0, len(requestPrefix)+5*32+8+1+8)
	raw = append(raw, requestPrefix...)
	for _, field := range [][32]byte{value.Network, value.Digest, value.TransitNode, value.Attachment, value.ClientKey} {
		raw = append(raw, field[:]...)
	}
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value.Epoch)
	raw = append(raw, encoded[:]...)
	raw = append(raw, value.Role)
	binary.BigEndian.PutUint64(encoded[:], uint64(value.NotAfter.Unix()))
	return append(raw, encoded[:]...), nil
}

func decodeRequest(raw []byte) (issuanceRequest, error) {
	if len(raw) != len(requestPrefix)+5*32+8+1+8 || string(raw[:len(requestPrefix)]) != requestPrefix {
		return issuanceRequest{}, errors.New("issuance request grammar is invalid")
	}
	offset := len(requestPrefix)
	result := issuanceRequest{}
	for _, field := range []*[32]byte{&result.Network, &result.Digest, &result.TransitNode, &result.Attachment, &result.ClientKey} {
		copy(field[:], raw[offset:offset+32])
		offset += 32
	}
	result.Epoch = binary.BigEndian.Uint64(raw[offset : offset+8])
	offset += 8
	result.Role = raw[offset]
	offset++
	seconds := binary.BigEndian.Uint64(raw[offset : offset+8])
	if seconds > uint64(^uint64(0)>>1) {
		return issuanceRequest{}, errors.New("issuance request expiry is invalid")
	}
	result.NotAfter = time.Unix(int64(seconds), 0).UTC()
	_, err := encodeRequest(result)
	return result, err
}

func clientCertificate() (tls.Certificate, error) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := &x509.Certificate{SerialNumber: new(big.Int).SetBytes(public[:8]), Subject: pkix.Name{CommonName: "r118-client"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Minute), KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	raw, err := x509.CreateCertificate(rand.Reader, template, template, public, private)
	if err != nil {
		return tls.Certificate{}, err
	}
	leaf, err := x509.ParseCertificate(raw)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{Certificate: [][]byte{raw}, PrivateKey: private, Leaf: leaf}, nil
}

func randomID() ([32]byte, error) {
	var value [32]byte
	_, err := rand.Read(value[:])
	return value, err
}

func marker(value byte) [32]byte {
	var result [32]byte
	for index := range result {
		result[index] = value
	}
	return result
}
