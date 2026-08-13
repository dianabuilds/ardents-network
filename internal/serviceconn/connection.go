package serviceconn

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
)

const challengeSize = 4 + 1 + 32 + 8 + 32
const proofSize = 4 + 1 + ed25519.SignatureSize

func (endpoint *Endpoint) connect(ctx context.Context, input Request) (Result, error) {
	if err := endpoint.consume(input.Session, input.Principal, "connection"); err != nil {
		return denied(err.Error())
	}
	credential, err := decodePublication(input.Publication, endpoint.authority, endpoint.network, input.At)
	if err != nil || input.Target == [32]byte{} || input.Target != credential.Target {
		return failed("service target authentication failure", "exact Service Target could not be authenticated", errors.New("target or publication mismatch"))
	}
	if err := validateStreams(input); err != nil {
		return failed("local authorization or policy denial", "bounded local stream input is invalid", err)
	}
	defer input.Route.Close()
	defer input.Application.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = input.Route.Close()
		_ = input.Application.Close()
	})
	defer stop()
	if err := authenticateInstance(input.Route, credential); err != nil {
		return failed("service target authentication failure", "current Service Instance proof failed", err)
	}
	accepted, received, err := exchangeExact(input.Application, input.Route, input.BytesEachDirection)
	if err != nil {
		return streamFailure(ctx, accepted, received, err)
	}
	return Result{Class: "clean service connection close", AuthenticatedTarget: credential.Target,
		Generation: credential.Generation, AcceptedBytes: input.BytesEachDirection,
		ReceivedBytes: input.BytesEachDirection}, nil
}

func (endpoint *Endpoint) accept(ctx context.Context, input Request) (Result, error) {
	if err := endpoint.consume(input.Session, input.Principal, "connection"); err != nil {
		return denied(err.Error())
	}
	if err := validateStreams(input); err != nil {
		return failed("local authorization or policy denial", "bounded local stream input is invalid", err)
	}
	endpoint.mu.Lock()
	if endpoint.current == nil || input.At.Unix() < endpoint.current.credential.NotBefore || input.At.Unix() >= endpoint.current.credential.NotAfter {
		endpoint.mu.Unlock()
		return failed("service unavailable", "no current published Service Instance is available", errors.New("publication is absent or expired"))
	}
	credential := endpoint.current.credential
	private := append(ed25519.PrivateKey(nil), endpoint.current.private...)
	endpoint.mu.Unlock()
	defer erase(private)
	defer input.Route.Close()
	defer input.Application.Close()
	stop := context.AfterFunc(ctx, func() {
		_ = input.Route.Close()
		_ = input.Application.Close()
	})
	defer stop()
	if err := proveInstance(input.Route, credential, private); err != nil {
		return failed("service target authentication failure", "incoming exact Target proof failed", err)
	}
	accepted, received, err := exchangeExact(input.Application, input.Route, input.BytesEachDirection)
	if err != nil {
		return streamFailure(ctx, accepted, received, err)
	}
	return Result{Class: "clean service connection close", AuthenticatedTarget: credential.Target,
		Generation: credential.Generation, AcceptedBytes: input.BytesEachDirection,
		ReceivedBytes: input.BytesEachDirection}, nil
}

func validateStreams(input Request) error {
	if input.Route == nil || input.Application == nil || input.BytesEachDirection == 0 || input.BytesEachDirection > maximumStream {
		return errors.New("stream or byte bound is missing")
	}
	return nil
}

func authenticateInstance(connection io.ReadWriter, credential Credential) error {
	challenge := make([]byte, challengeSize)
	copy(challenge[:4], "ASCH")
	challenge[4] = 1
	copy(challenge[5:37], credential.Target[:])
	binary.BigEndian.PutUint64(challenge[37:45], credential.Generation)
	if _, err := rand.Read(challenge[45:]); err != nil {
		return err
	}
	if err := writeAll(connection, challenge); err != nil {
		return err
	}
	proof := make([]byte, proofSize)
	if _, err := io.ReadFull(connection, proof); err != nil {
		return err
	}
	if string(proof[:4]) != "ASPR" || proof[4] != 1 ||
		!ed25519.Verify(ed25519.PublicKey(credential.InstancePublic[:]), proofMessage(challenge), proof[5:]) {
		return errors.New("instance proof is invalid")
	}
	return nil
}

func proveInstance(connection io.ReadWriter, credential Credential, private ed25519.PrivateKey) error {
	challenge := make([]byte, challengeSize)
	if _, err := io.ReadFull(connection, challenge); err != nil {
		return err
	}
	if string(challenge[:4]) != "ASCH" || challenge[4] != 1 ||
		!equal32(challenge[5:37], credential.Target) || binary.BigEndian.Uint64(challenge[37:45]) != credential.Generation {
		return errors.New("connection challenge does not bind current Target and generation")
	}
	proof := make([]byte, proofSize)
	copy(proof[:4], "ASPR")
	proof[4] = 1
	copy(proof[5:], ed25519.Sign(private, proofMessage(challenge)))
	return writeAll(connection, proof)
}

func proofMessage(challenge []byte) []byte {
	message := make([]byte, 0, 30+len(challenge))
	message = append(message, "ardents-h3-instance-proof-v1\x00"...)
	message = append(message, challenge...)
	return message
}

func equal32(value []byte, expected [32]byte) bool {
	if len(value) != len(expected) {
		return false
	}
	for index := range expected {
		if value[index] != expected[index] {
			return false
		}
	}
	return true
}
