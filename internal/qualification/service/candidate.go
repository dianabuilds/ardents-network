package service

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net"
	"strings"
)

func validateCandidate(input candidate) error {
	if input.Target != targetFor(input.AuthorityPublic) {
		return errors.New("service Target does not bind the declared Authority")
	}
	commitment := make([]byte, 0, 32*6)
	credentialDigest := sha256.Sum256(append(credentialJSON(input.Generations[0].Credential),
		credentialJSON(input.Generations[1].Credential)...))
	for _, field := range [][32]byte{input.RouteManifestDigest, input.NetworkID, input.AuthorityPublic,
		input.IntroductionPublic, input.Target, credentialDigest} {
		commitment = append(commitment, field[:]...)
	}
	manifest := sha256.Sum256(commitment)
	if input.ManifestDigest != hexDigest(manifest) {
		return errors.New("service manifest does not bind its authoritative inputs")
	}
	first, second := input.Generations[0], input.Generations[1]
	if first.Generation != 1 || second.Generation != 2 || first.Credential.Generation != 1 ||
		second.Credential.Generation != 2 || first.Credential.InstancePublic == second.Credential.InstancePublic ||
		first.ClientApplication.SendSeed == second.ClientApplication.SendSeed ||
		first.PublisherApplication.SendSeed == second.PublisherApplication.SendSeed {
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
	if !validIntroductionReceipt(generation.IntroductionAcknowledgement, input, credential) {
		return errors.New("introduction acknowledgement is not bound to the publication")
	}
	for _, endpoint := range []endpointEvidence{generation.ClientEndpoint, generation.PublisherEndpoint} {
		if endpoint.Class != "clean service connection close" || endpoint.AuthenticatedTarget != input.Target ||
			endpoint.Generation != generation.Generation || endpoint.AcceptedBytes != 64<<10 || endpoint.ReceivedBytes != 64<<10 ||
			endpoint.ConnectionCanary == [32]byte{} {
			return errors.New("endpoint Service Connection result violates the frozen contract")
		}
	}
	if generation.ClientEndpoint.ConnectionCanary != generation.PublisherEndpoint.ConnectionCanary {
		return errors.New("connection canary observations do not match")
	}
	client, publisher := generation.ClientApplication, generation.PublisherApplication
	if client.Schema != "ardents-h3-stream-application-v1" || publisher.Schema != client.Schema ||
		client.Role != "client" || publisher.Role != "publisher" ||
		client.Terminal != "success" || publisher.Terminal != "success" || client.SentBytes != 64<<10 ||
		client.ReceivedBytes != 64<<10 || publisher.SentBytes != 64<<10 || publisher.ReceivedBytes != 64<<10 ||
		client.SentDigest != publisher.ReceivedDigest || publisher.SentDigest != client.ReceivedDigest ||
		client.ResultClass != "clean service connection close" || publisher.ResultClass != client.ResultClass ||
		client.AuthenticatedTarget != input.Target || publisher.AuthenticatedTarget != input.Target ||
		client.SendSeed == [32]byte{} || publisher.SendSeed == [32]byte{} || client.SendSeed != publisher.ExpectSeed ||
		publisher.SendSeed != client.ExpectSeed || sha256.Sum256(expectedWorkload(client.SendSeed)) != client.SentDigest ||
		sha256.Sum256(expectedWorkload(publisher.SendSeed)) != publisher.SentDigest {
		return errors.New("opaque Application stream length, order, or digest differs")
	}
	for _, role := range generation.Roles {
		if role.Terminal != "success" || !role.Cleanup {
			return errors.New("route actor did not terminate successfully and cleanly")
		}
	}
	if err := validateRouteChain(generation.Roles, input.Topology); err != nil {
		return err
	}
	hostile := generation.HostileSibling
	if !validContainerID(hostile.RuntimeID) || hostile.Running || hostile.ExitCode == 0 || len(hostile.MountDestinations) != 0 ||
		!strings.Contains(hostile.Output, "dial unix /run/ardents/not-granted/app.sock") ||
		!strings.Contains(hostile.Output, "no such file or directory") {
		return errors.New("hostile sibling rejection is not supported by retained process and mount observations")
	}
	seenHostile := false
	for _, identity := range generation.ContainerIDs {
		seenHostile = seenHostile || identity == hostile.RuntimeID
	}
	if !seenHostile {
		return errors.New("hostile sibling observation is not bound to the retained container set")
	}
	return nil
}

func validateRouteChain(roles []roleEvidence, topology string) error {
	byRole := make(map[string]roleEvidence, len(roles))
	for _, role := range roles {
		byRole[role.Role] = role
	}
	client := byRole["client"]
	if len(client.Positions) != 4 {
		return errors.New("client Route observation does not retain four selected positions")
	}
	chain := []string{"initiator", "introduction", "rendezvous", "responder"}
	epoch := client.EpochDigest
	if epoch == [32]byte{} {
		return errors.New("client Route observation omits its epoch digest")
	}
	for index, name := range chain {
		position, actor := client.Positions[index], byRole[name]
		host, port, endpointErr := net.SplitHostPort(position.Endpoint)
		if position.Role != name || position.NodeID == [32]byte{} || position.Endpoint == "" ||
			actor.NodeID != position.NodeID || actor.EpochDigest != epoch || endpointErr != nil || port == "" ||
			host != topologyIPv4([]byte(topology), name) {
			return errors.New("route observation does not bind the selected ordered position: " + name)
		}
		expectedNext := byRole["publisher"].NodeID
		if index+1 < len(chain) {
			expectedNext = client.Positions[index+1].NodeID
		}
		if actor.NextNodeID != expectedNext {
			return errors.New("route observation contains a direct or shortened role transition: " + name)
		}
	}
	if byRole["publisher"].NodeID == [32]byte{} || byRole["publisher"].EpochDigest != epoch {
		return errors.New("route observation omits the publisher identity")
	}
	if !sameTransfer(client, byRole["publisher"]) {
		return errors.New("client and publisher Route transfer observations differ")
	}
	firstNode := byRole[chain[0]]
	for _, name := range chain[1:] {
		if !sameTransfer(firstNode, byRole[name]) {
			return errors.New("Route actor transfer continuity differs: " + name)
		}
	}
	return nil
}

func sameTransfer(left, right roleEvidence) bool {
	return left.OpaqueBytes == right.OpaqueBytes && left.OpaqueDigest == right.OpaqueDigest &&
		left.ReverseOpaqueBytes == right.ReverseOpaqueBytes && left.ReverseOpaqueDigest == right.ReverseOpaqueDigest
}

func validContainerID(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, character := range value {
		if !strings.ContainsRune("0123456789abcdef", character) {
			return false
		}
	}
	return true
}

func expectedWorkload(seed [32]byte) []byte {
	value := make([]byte, 0, 64<<10)
	for counter := uint64(0); len(value) < 64<<10; counter++ {
		block := make([]byte, 40)
		copy(block, seed[:])
		binary.BigEndian.PutUint64(block[32:], counter)
		digest := sha256.Sum256(block)
		value = append(value, digest[:]...)
	}
	return value[:64<<10]
}

func validIntroductionReceipt(raw []byte, input candidate, credential publicCredential) bool {
	if len(raw) != 213 || string(raw[:4]) != "ASIA" || raw[4] != 1 ||
		!bytes.Equal(raw[5:37], credential.Target[:]) || binary.BigEndian.Uint64(raw[37:45]) != credential.Generation ||
		binary.BigEndian.Uint64(raw[45:53]) != uint64(credential.NotAfter) || !bytes.Equal(raw[53:85], input.NetworkID[:]) ||
		bytes.Equal(raw[85:117], make([]byte, 32)) || bytes.Equal(raw[117:149], make([]byte, 32)) {
		return false
	}
	message := append([]byte("ardents-h3-introduction-ack-v1\x00"), raw[:149]...)
	return ed25519.Verify(ed25519.PublicKey(input.IntroductionPublic[:]), message, raw[149:])
}

func credentialJSON(value publicCredential) []byte {
	raw, _ := json.Marshal(value)
	return raw
}

func hexDigest(value [32]byte) string { return hex.EncodeToString(value[:]) }

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
