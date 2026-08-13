package service

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
)

func validateCandidate(input candidate) error {
	if input.Target != targetFor(input.AuthorityPublic) {
		return errors.New("service Target does not bind the declared Authority")
	}
	first, second := input.Generations[0], input.Generations[1]
	if first.Generation != 1 || second.Generation != 2 || first.Credential.Generation != 1 ||
		second.Credential.Generation != 2 || first.Credential.InstancePublic == second.Credential.InstancePublic {
		return errors.New("routine migration generation or Instance Key is wrong")
	}
	for _, generation := range input.Generations {
		if err := validateGeneration(input, generation); err != nil {
			return err
		}
	}
	for name, passed := range input.Negatives {
		if !passed {
			return errors.New("negative case failed: " + name)
		}
	}
	for name, absent := range input.ShortcutsAbsent {
		if !absent {
			return errors.New("forbidden path was present: " + name)
		}
	}
	for name, cleaned := range input.Cleanup {
		if !cleaned {
			return errors.New("cleanup failed: " + name)
		}
	}
	return nil
}

func validateGeneration(input candidate, generation generationEvidence) error {
	credential := generation.Credential
	if credential.Target != input.Target || credential.AuthorityPublic != input.AuthorityPublic ||
		credential.NetworkID != input.NetworkID || credential.Generation != generation.Generation ||
		credential.InstancePublic == [32]byte{} || credential.NotBefore >= credential.NotAfter ||
		credential.Capabilities&3 != 3 || !ed25519.Verify(ed25519.PublicKey(input.AuthorityPublic[:]),
		credentialBody(credential), credential.Signature[:]) {
		return errors.New("credential or exact Target authentication evidence failed")
	}
	for _, endpoint := range []endpointEvidence{generation.ClientEndpoint, generation.PublisherEndpoint} {
		if endpoint.Class != "clean service connection close" || endpoint.AuthenticatedTarget != input.Target ||
			endpoint.Generation != generation.Generation || endpoint.AcceptedBytes != 64<<10 || endpoint.ReceivedBytes != 64<<10 {
			return errors.New("endpoint Service Connection result violates the frozen contract")
		}
	}
	client, publisher := generation.ClientApplication, generation.PublisherApplication
	if client.Schema != "ardents-h3-stream-application-v1" || publisher.Schema != client.Schema ||
		client.Role != "client" || publisher.Role != "publisher" ||
		client.Terminal != "success" || publisher.Terminal != "success" || client.SentBytes != 64<<10 ||
		client.ReceivedBytes != 64<<10 || publisher.SentBytes != 64<<10 || publisher.ReceivedBytes != 64<<10 ||
		client.SentDigest != publisher.ReceivedDigest || publisher.SentDigest != client.ReceivedDigest {
		return errors.New("opaque Application stream length, order, or digest differs")
	}
	for _, role := range generation.Roles {
		if role.Terminal != "success" || !role.Cleanup {
			return errors.New("route actor did not terminate successfully and cleanly")
		}
	}
	return nil
}

func targetFor(authority [32]byte) [32]byte {
	return sha256.Sum256(append([]byte("ardents-h3-service-target-v1\x00"), authority[:]...))
}

func credentialBody(value publicCredential) []byte {
	encoded := make([]byte, 4+1+32+32+32+8+8+8+32+4)
	copy(encoded[:4], "ASCR")
	encoded[4] = 1
	offset := 5
	for _, field := range [][32]byte{value.AuthorityPublic, value.Target, value.InstancePublic} {
		copy(encoded[offset:offset+32], field[:])
		offset += 32
	}
	binary.BigEndian.PutUint64(encoded[offset:offset+8], value.Generation)
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.NotBefore))
	offset += 8
	binary.BigEndian.PutUint64(encoded[offset:offset+8], uint64(value.NotAfter))
	offset += 8
	copy(encoded[offset:offset+32], value.NetworkID[:])
	offset += 32
	binary.BigEndian.PutUint32(encoded[offset:], value.Capabilities)
	return encoded
}
