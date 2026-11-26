package publication

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"testing"
)

func TestServicePublicContractIndependentBytes(t *testing.T) {
	view := RequestView{NetworkID: [32]byte{3}, InstancePublic: [32]byte{4}, NotBefore: 1900000000, NotAfter: 1900003600}
	request := append([]byte("ardents-service-instance-request-v3\x00"), view.NetworkID[:]...)
	request = append(request, view.InstancePublic[:]...)
	request = binary.BigEndian.AppendUint64(request, 1900000000)
	request = binary.BigEndian.AppendUint64(request, 1900003600)
	view.Commitment = sha256.Sum256(request)
	request = append(request, view.Commitment[:]...)
	if !bytes.Equal(request, EncodeServiceRequest(view)) || ServiceRequestCommitment(view) != view.Commitment || sha256.Sum256(request) == view.Commitment {
		t.Fatal("request bytes or separate approval digest changed")
	}
	parsed, err := ParseServiceRequest(request)
	if err != nil || parsed != view {
		t.Fatal(parsed, err)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	var public [32]byte
	copy(public[:], private[32:])
	target := sha256.Sum256(append([]byte("ardents-service-target-v3\x00"), public[:]...))
	body := append([]byte{0, 3}, public[:]...)
	body = append(body, target[:]...)
	body = append(body, view.InstancePublic[:]...)
	for _, value := range []uint64{7, 1900000000, 1900003600} {
		body = binary.BigEndian.AppendUint64(body, value)
	}
	body = append(body, view.NetworkID[:]...)
	body = binary.BigEndian.AppendUint32(body, 3)
	transcript, err := ServiceCredentialTranscript(view, public, 7)
	if err != nil || !bytes.Equal(body, transcript) {
		t.Fatal("Credential transcript changed", err)
	}
	credential := append(body, ed25519.Sign(private, body)...)
	expected := append([]byte("ardents-service-instance-response-v3\x00"), view.Commitment[:]...)
	expected = append(expected, credential...)
	response, err := BuildServiceResponse(request, credential)
	if err != nil || !bytes.Equal(response, expected) {
		t.Fatal("response bytes changed", err)
	}
	verified, err := VerifyServiceResponse(expected, view)
	if err != nil || verified.Delegation().Generation != 7 || verified.Delegation().Target != target {
		t.Fatal(verified, err)
	}
	for _, offset := range []int{0, len(requestDomain), len(requestDomain) + 32, len(request) - 1} {
		bad := bytes.Clone(request)
		bad[offset] ^= 1
		if _, err := ParseServiceRequest(bad); err == nil {
			t.Fatal("altered request accepted", offset)
		}
	}
	for _, offset := range []int{0, len(responseDomain), len(responseDomain) + 32, len(expected) - 1} {
		bad := bytes.Clone(expected)
		bad[offset] ^= 1
		if _, err := VerifyServiceResponse(bad, view); err == nil {
			t.Fatal("altered response accepted", offset)
		}
	}
	if _, err := VerifyServiceResponse(expected, RequestView{}); err == nil {
		t.Fatal("unverified zero request accepted")
	}
	for _, raw := range [][]byte{nil, request[:len(request)-1], append(bytes.Clone(request), 0)} {
		if _, err := ParseServiceRequest(raw); err == nil {
			t.Fatal("nonexact length accepted")
		}
	}
	if _, err := ServiceCredentialTranscript(view, public, 0); err == nil {
		t.Fatal("zero generation accepted")
	}
	// Independently signed controls distinguish field admission from a merely
	// damaged signature. Exact request verification must reject each one.
	for _, field := range []string{"generation", "capabilities", "instance", "network", "validity"} {
		t.Run(field, func(t *testing.T) {
			badBody := bytes.Clone(body)
			switch field {
			case "generation":
				binary.BigEndian.PutUint64(badBody[98:106], 0)
			case "capabilities":
				binary.BigEndian.PutUint32(badBody[154:158], 7)
			case "instance":
				badBody[66] ^= 1
			case "network":
				badBody[122] ^= 1
			case "validity":
				binary.BigEndian.PutUint64(badBody[114:122], 1900003599)
			}
			badCredential := append(badBody, ed25519.Sign(private, badBody)...)
			badResponse := append([]byte("ardents-service-instance-response-v3\x00"), view.Commitment[:]...)
			badResponse = append(badResponse, badCredential...)
			if _, err := VerifyServiceResponse(badResponse, view); err == nil {
				t.Fatal("independently signed foreign or unsupported field accepted")
			}
			if _, err := BuildServiceResponse(request, badCredential); err == nil {
				t.Fatal("response producer framed a refused Credential")
			}
		})
	}
}
