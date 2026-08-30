package instance

import (
	"bytes"
	"time"

	"github.com/dianabuilds/ardents-network/internal/service/publication"
)

const responseDomain = "ardents-service-instance-response-v1\x00"

// ResponseView is the public Authority response for one exact pending request.
type ResponseView struct {
	RequestCommitment [32]byte
	Credential        publication.Credential
}

// BuildResponse binds one signed Credential to its exact canonical request.
func BuildResponse(request []byte, credential publication.Credential) ([]byte, error) {
	view, err := ParseRequest(request)
	if err != nil || !credentialMatchesRequest(credential, view) {
		return nil, ErrInvalid
	}
	if err := publication.Validate(credential, credential.AuthorityPublic, view.NetworkID,
		time.Unix(view.NotBefore, 0).UTC(), publication.CapabilityPublish|publication.CapabilityConnect); err != nil {
		return nil, ErrInvalid
	}
	credentialBytes, err := publication.EncodeCredential(credential)
	if err != nil {
		return nil, ErrInvalid
	}
	response := make([]byte, len(responseDomain)+32+len(credentialBytes))
	offset := copy(response, responseDomain)
	offset += copy(response[offset:], view.Commitment[:])
	copy(response[offset:], credentialBytes)
	return response, nil
}

// ParseResponse accepts only a self-consistent signed response. The pending
// Root performs the additional exact-request and at-most-once checks.
func ParseResponse(raw []byte) (ResponseView, error) {
	minimum := len(responseDomain) + 32 + 1
	if len(raw) < minimum || !bytes.Equal(raw[:len(responseDomain)], []byte(responseDomain)) {
		return ResponseView{}, ErrInvalid
	}
	var response ResponseView
	offset := len(responseDomain)
	copy(response.RequestCommitment[:], raw[offset:offset+32])
	credential, err := publication.DecodeCredential(raw[offset+32:])
	if err != nil || response.RequestCommitment == ([32]byte{}) {
		return ResponseView{}, ErrInvalid
	}
	if err := publication.Validate(credential, credential.AuthorityPublic, credential.NetworkID,
		time.Unix(credential.NotBefore, 0).UTC(), publication.CapabilityPublish|publication.CapabilityConnect); err != nil {
		return ResponseView{}, ErrInvalid
	}
	response.Credential = credential
	return response, nil
}

func credentialMatchesRequest(credential publication.Credential, request RequestView) bool {
	return credential.InstancePublic == request.InstancePublic &&
		credential.IntroductionHPKEPublic == request.IntroductionPublic &&
		credential.NetworkID == request.NetworkID && credential.NotBefore == request.NotBefore &&
		credential.NotAfter == request.NotAfter && credential.Generation != 0 &&
		credential.Capabilities == publication.CapabilityPublish|publication.CapabilityConnect
}
