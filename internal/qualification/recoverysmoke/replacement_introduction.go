package recoverysmoke

import (
	"encoding/json"
	"errors"
)

func introductionSetupEvidence(clientRaw, introductionRaw, serviceRaw []byte) (
	[32]byte, uint32, uint64, [32]byte, error) {
	client, clientCount, clientErr := setupReceipt(clientRaw, "client")
	service, serviceCount, serviceErr := setupReceipt(serviceRaw, "publisher")
	opaqueBytes, opaqueDigest, relayCount, relayErr := opaqueSetupReceipt(introductionRaw)
	if clientErr != nil || serviceErr != nil || relayErr != nil {
		return [32]byte{}, 0, 0, [32]byte{}, errors.Join(clientErr, serviceErr, relayErr)
	}
	if clientCount != 1 || serviceCount != 1 || relayCount != 1 || client.receipt == [32]byte{} ||
		client.receipt != service.receipt || client.proof != service.proof || client.attachment == 0 ||
		opaqueBytes == 0 || opaqueDigest == [32]byte{} {
		return [32]byte{}, 0, 0, [32]byte{}, errors.New("fresh opaque sealed Introduction setup evidence is incomplete")
	}
	return client.receipt, client.attachment, opaqueBytes, opaqueDigest, nil
}

type setupObservation struct {
	receipt    [32]byte
	proof      introductionProof
	attachment uint32
}

type setupEvidence struct {
	Kind                     string            `json:"kind"`
	Role                     string            `json:"role"`
	Attachment               uint32            `json:"attachment"`
	IntroductionSetupReceipt [32]byte          `json:"introduction_setup_receipt"`
	IntroductionSetup        introductionProof `json:"introduction_setup"`
	IntroductionOpaqueBytes  uint64            `json:"introduction_opaque_bytes"`
	IntroductionOpaqueDigest [32]byte          `json:"introduction_opaque_digest"`
}

func setupReceipt(raw []byte, role string) (setupObservation, int, error) {
	var result setupObservation
	count := 0
	for _, line := range splitLines(raw) {
		var value setupEvidence
		if err := json.Unmarshal(line, &value); err != nil {
			return setupObservation{}, 0, errors.Join(err, errors.New("decode "+role+" sealed setup evidence"))
		}
		if value.Kind == "complete" && value.Role == role && value.IntroductionSetupReceipt != [32]byte{} {
			result = setupObservation{receipt: value.IntroductionSetupReceipt,
				proof: value.IntroductionSetup, attachment: value.Attachment}
			count++
		}
	}
	return result, count, nil
}

func opaqueSetupReceipt(raw []byte) (uint64, [32]byte, int, error) {
	var bytes uint64
	var digest [32]byte
	count := 0
	for _, line := range splitLines(raw) {
		var value setupEvidence
		if err := json.Unmarshal(line, &value); err != nil {
			return 0, [32]byte{}, 0, errors.Join(err, errors.New("decode opaque Introduction relay evidence"))
		}
		if value.Kind == "complete" && value.Role == "introduction" && value.IntroductionOpaqueBytes != 0 {
			if value.IntroductionSetupReceipt != [32]byte{} || value.IntroductionSetup != (introductionProof{}) {
				return 0, [32]byte{}, 0, errors.New("introduction relay retained sealed plaintext")
			}
			bytes, digest, count = value.IntroductionOpaqueBytes, value.IntroductionOpaqueDigest, count+1
		}
	}
	return bytes, digest, count, nil
}
