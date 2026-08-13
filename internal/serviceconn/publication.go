package serviceconn

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"
)

const publicationSize = 4 + 1 + credentialSize + 32 + ed25519.SignatureSize
const acknowledgementBodySize = 4 + 1 + 32 + 8 + 8 + 32 + 32 + 32
const acknowledgementSize = acknowledgementBodySize + ed25519.SignatureSize

type currentPublication struct {
	credential Credential
	private    ed25519.PrivateKey
	encoded    []byte
}

func (endpoint *Endpoint) publish(ctx context.Context, input Request) (Result, error) {
	if err := endpoint.consume(input.Session, input.Principal, "administration"); err != nil {
		return denied(err.Error())
	}
	if err := validateCredential(input.Credential, endpoint.authority, endpoint.network, input.At, publishCapability|connectCapability); err != nil {
		return failed("service target authentication failure", "Service Credential is not valid for publication", err)
	}
	if len(input.IntroductionAcknowledgement) == 0 && input.IntroductionSocket != "" {
		acknowledgement, err := requestIntroductionAcknowledgement(ctx, input.IntroductionSocket,
			input.Credential, endpoint.broker, endpoint.resources)
		if err != nil {
			return failed("service unavailable", "Introduction acknowledgement request failed", err)
		}
		input.IntroductionAcknowledgement = acknowledgement
	}
	if !validAcknowledgement(input.IntroductionAcknowledgement, input.Credential, endpoint.network,
		endpoint.broker, endpoint.introduction) {
		return failed("service unavailable", "fresh Introduction publication acknowledgement is absent", errors.New("publication not acknowledged"))
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
	if endpoint.current != nil {
		erase(endpoint.current.private)
	}
	endpoint.current = &currentPublication{credential: input.Credential, private: private, encoded: encoded}
	endpoint.lastGeneration = input.Credential.Generation
	if err := writeGeneration(endpoint.generationStateFile, endpoint.lastGeneration); err != nil {
		erase(endpoint.current.private)
		endpoint.current = nil
		return failed("service unavailable", "exclusive generation state could not be retained", err)
	}
	endpoint.resources("control-file", 1)
	return Result{Class: "published", Publication: append([]byte(nil), encoded...),
		IntroductionReceipt:         sha256.Sum256(input.IntroductionAcknowledgement),
		IntroductionAcknowledgement: append([]byte(nil), input.IntroductionAcknowledgement...),
		AuthenticatedTarget:         input.Credential.Target, Generation: input.Credential.Generation}, nil
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

func encodePublication(credential Credential, acknowledgement []byte, private ed25519.PrivateKey) []byte {
	encoded := make([]byte, publicationSize)
	copy(encoded[:4], "ASPB")
	encoded[4] = 1
	copy(encoded[5:5+credentialSize], encodeCredential(credential))
	digest := sha256.Sum256(acknowledgement)
	copy(encoded[5+credentialSize:5+credentialSize+32], digest[:])
	signature := ed25519.Sign(private, publicationMessage(credential, digest))
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

func validAcknowledgement(raw []byte, credential Credential, network, broker, introduction [32]byte) bool {
	if len(raw) != acknowledgementSize || string(raw[:4]) != "ASIA" || raw[4] != 1 ||
		!equal32(raw[5:37], credential.Target) || binary.BigEndian.Uint64(raw[37:45]) != credential.Generation ||
		binary.BigEndian.Uint64(raw[45:53]) != uint64(credential.NotAfter) || !equal32(raw[53:85], network) ||
		!equal32(raw[85:117], broker) || bytes.Equal(raw[117:149], make([]byte, 32)) {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(introduction[:]), acknowledgementMessage(raw[:acknowledgementBodySize]),
		raw[acknowledgementBodySize:])
}

func acknowledgementMessage(body []byte) []byte {
	return append([]byte("ardents-h3-introduction-ack-v1\x00"), body...)
}

func erase(value []byte) {
	for index := range value {
		value[index] = 0
	}
}

func (endpoint *Endpoint) retire(generation uint64) {
	endpoint.mu.Lock()
	defer endpoint.mu.Unlock()
	if endpoint.current != nil && endpoint.current.credential.Generation == generation {
		erase(endpoint.current.private)
		endpoint.current = nil
	}
}
