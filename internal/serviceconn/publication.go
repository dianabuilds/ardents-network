package serviceconn

import (
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"
)

const publicationSize = 4 + 1 + credentialSize + 32 + ed25519.SignatureSize

type currentPublication struct {
	credential Credential
	private    ed25519.PrivateKey
	encoded    []byte
}

func (endpoint *Endpoint) publish(input Request) (Result, error) {
	if err := endpoint.consume(input.Session, input.Principal, "administration"); err != nil {
		return denied(err.Error())
	}
	if input.IntroductionAcknowledgement == [32]byte{} {
		return failed("service unavailable", "fresh Introduction publication acknowledgement is absent", errors.New("publication not acknowledged"))
	}
	if err := validateCredential(input.Credential, endpoint.authority, endpoint.network, input.At, publishCapability|connectCapability); err != nil {
		return failed("service target authentication failure", "Service Credential is not valid for publication", err)
	}
	if len(input.InstancePrivate) != ed25519.PrivateKeySize ||
		!input.InstancePrivate.Public().(ed25519.PublicKey).Equal(ed25519.PublicKey(input.Credential.InstancePublic[:])) {
		return failed("service target authentication failure", "matching Instance Key possession was not proved", errors.New("instance Key mismatch"))
	}
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if input.Credential.Generation <= endpoint.lastGeneration {
		return failed("service target authentication failure", "exclusive Instance generation is stale or conflicting", errors.New("generation is not higher"))
	}
	encoded := encodePublication(input.Credential, input.IntroductionAcknowledgement, input.InstancePrivate)
	private := append(ed25519.PrivateKey(nil), input.InstancePrivate...)
	endpoint.current = &currentPublication{credential: input.Credential, private: private, encoded: encoded}
	endpoint.lastGeneration = input.Credential.Generation
	return Result{Class: "published", Publication: append([]byte(nil), encoded...),
		AuthenticatedTarget: input.Credential.Target, Generation: input.Credential.Generation}, nil
}

func (endpoint *Endpoint) unpublish(input Request) (Result, error) {
	if err := endpoint.consume(input.Session, input.Principal, "administration"); err != nil {
		return denied(err.Error())
	}
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if endpoint.current == nil {
		return failed("service unavailable", "Service is not currently published", errors.New("no current publication"))
	}
	erase(endpoint.current.private)
	generation, target := endpoint.current.credential.Generation, endpoint.current.credential.Target
	endpoint.current = nil
	return Result{Class: "unpublished", AuthenticatedTarget: target, Generation: generation}, nil
}

func encodePublication(credential Credential, acknowledgement [32]byte, private ed25519.PrivateKey) []byte {
	encoded := make([]byte, publicationSize)
	copy(encoded[:4], "ASPB")
	encoded[4] = 1
	copy(encoded[5:5+credentialSize], encodeCredential(credential))
	copy(encoded[5+credentialSize:5+credentialSize+32], acknowledgement[:])
	signature := ed25519.Sign(private, publicationMessage(credential, acknowledgement))
	copy(encoded[5+credentialSize+32:], signature)
	return encoded
}

func decodePublication(encoded []byte, authority, network [32]byte, at time.Time) (Credential, error) {
	if len(encoded) != publicationSize || string(encoded[:4]) != "ASPB" || encoded[4] != 1 {
		return Credential{}, errors.New("publication is malformed or oversized")
	}
	credential, err := decodeCredential(encoded[5 : 5+credentialSize])
	if err != nil || validateCredential(credential, authority, network, at, connectCapability) != nil {
		return Credential{}, errors.New("publication credential is invalid")
	}
	var acknowledgement [32]byte
	copy(acknowledgement[:], encoded[5+credentialSize:5+credentialSize+32])
	if acknowledgement == [32]byte{} || !ed25519.Verify(ed25519.PublicKey(credential.InstancePublic[:]),
		publicationMessage(credential, acknowledgement), encoded[5+credentialSize+32:]) {
		return Credential{}, errors.New("publication lacks current Instance possession")
	}
	return credential, nil
}

func publicationMessage(credential Credential, acknowledgement [32]byte) []byte {
	digest := sha256.Sum256(encodeCredential(credential))
	message := make([]byte, 0, 30+32+32)
	message = append(message, "ardents-h3-publication-v1\x00"...)
	message = append(message, digest[:]...)
	message = append(message, acknowledgement[:]...)
	return message
}

func erase(value []byte) {
	for index := range value {
		value[index] = 0
	}
}
