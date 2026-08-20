package nameclaim

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

const maximumProofWireBytes = 2 << 10

type proofWire struct {
	Network                []byte      `json:"network"`
	Epoch                  uint64      `json:"epoch"`
	Rule                   string      `json:"rule"`
	CutoffOffset           int64       `json:"cutoff_offset"`
	InputRoot              []byte      `json:"input_root"`
	InputLength            uint32      `json:"input_length"`
	MaterializationRoot    []byte      `json:"materialization_root"`
	MaterializationLength  uint32      `json:"materialization_length"`
	RejectionRoot          []byte      `json:"rejection_root"`
	RejectionLength        uint32      `json:"rejection_length"`
	MaterializationOrdinal uint32      `json:"materialization_ordinal"`
	MaterializationPath    [][]byte    `json:"materialization_path"`
	Claims                 []claimWire `json:"claims"`
	SignerIDs              [][]byte    `json:"signer_ids"`
	Signatures             [][]byte    `json:"signatures"`
	AlternateSets          []proofWire `json:"alternate_sets"`
}

type claimWire struct {
	Ordinal         uint32   `json:"ordinal"`
	Name            string   `json:"name"`
	Secret          []byte   `json:"secret"`
	Authority       []byte   `json:"authority"`
	Commitment      []byte   `json:"commitment"`
	AdmissionDigest []byte   `json:"admission_digest"`
	InputPath       [][]byte `json:"input_path"`
	Signature       []byte   `json:"signature"`
}

// CanonicalProof encodes proof when raw is nil, or strictly decodes raw into
// proof otherwise. Both directions enforce the same bounded canonical wire.
func CanonicalProof(raw []byte, proof *Proof) ([]byte, error) {
	if proof == nil {
		return nil, errors.New("claim proof destination is missing")
	}
	if raw == nil {
		wire := proofToWire(*proof)
		encoded, err := json.Marshal(wire)
		if err != nil || len(encoded) == 0 || len(encoded) > maximumProofWireBytes {
			return nil, errors.New("claim proof wire exceeds its bound")
		}
		return encoded, nil
	}
	if len(raw) == 0 || len(raw) > maximumProofWireBytes {
		return nil, errors.New("claim proof wire size is invalid")
	}
	var wire proofWire
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&wire) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		return nil, errors.New("claim proof wire is malformed")
	}
	canonical, err := json.Marshal(wire)
	if err != nil || !bytes.Equal(canonical, raw) {
		return nil, errors.New("claim proof wire is non-canonical")
	}
	decoded, err := wireToProof(wire)
	if err != nil {
		return nil, err
	}
	*proof = decoded
	return append([]byte(nil), raw...), nil
}

func proofToWire(proof Proof) proofWire {
	wire := proofWire{Network: proof.Network[:], Epoch: proof.Epoch, Rule: proof.Rule,
		CutoffOffset: proof.CutoffOffset, InputRoot: proof.InputRoot[:], InputLength: proof.InputLength,
		MaterializationRoot: proof.MaterializationRoot[:], MaterializationLength: proof.MaterializationLength,
		RejectionRoot: proof.RejectionRoot[:], RejectionLength: proof.RejectionLength,
		MaterializationOrdinal: proof.MaterializationOrdinal}
	for _, value := range proof.MaterializationPath {
		wire.MaterializationPath = append(wire.MaterializationPath, value[:])
	}
	for _, claim := range proof.Claims {
		item := claimWire{Ordinal: claim.Ordinal, Name: claim.Name, Secret: claim.Secret[:], Authority: claim.Authority[:],
			Commitment: claim.Commitment[:], AdmissionDigest: claim.AdmissionDigest[:], Signature: claim.Signature[:]}
		for _, value := range claim.InputPath {
			item.InputPath = append(item.InputPath, value[:])
		}
		wire.Claims = append(wire.Claims, item)
	}
	for _, value := range proof.SignerIDs {
		wire.SignerIDs = append(wire.SignerIDs, value[:])
	}
	for _, value := range proof.Signatures {
		wire.Signatures = append(wire.Signatures, append([]byte(nil), value...))
	}
	for _, alternate := range proof.AlternateSets {
		wire.AlternateSets = append(wire.AlternateSets, proofToWire(alternate))
	}
	return wire
}

func wireToProof(wire proofWire) (Proof, error) {
	proof := Proof{Epoch: wire.Epoch, Rule: wire.Rule, CutoffOffset: wire.CutoffOffset,
		InputLength: wire.InputLength, MaterializationLength: wire.MaterializationLength,
		RejectionLength: wire.RejectionLength, MaterializationOrdinal: wire.MaterializationOrdinal}
	if !copy32(&proof.Network, wire.Network) || !copy32(&proof.InputRoot, wire.InputRoot) ||
		!copy32(&proof.MaterializationRoot, wire.MaterializationRoot) || !copy32(&proof.RejectionRoot, wire.RejectionRoot) {
		return Proof{}, errors.New("claim proof fixed field size is invalid")
	}
	for _, raw := range wire.MaterializationPath {
		var value [32]byte
		if !copy32(&value, raw) {
			return Proof{}, errors.New("claim materialization path is invalid")
		}
		proof.MaterializationPath = append(proof.MaterializationPath, value)
	}
	for _, item := range wire.Claims {
		claim := Claim{Ordinal: item.Ordinal, Name: item.Name}
		if !copy32(&claim.Secret, item.Secret) || !copy32(&claim.Authority, item.Authority) ||
			!copy32(&claim.Commitment, item.Commitment) || !copy32(&claim.AdmissionDigest, item.AdmissionDigest) ||
			len(item.Signature) != len(claim.Signature) {
			return Proof{}, errors.New("claim reveal fixed field size is invalid")
		}
		copy(claim.Signature[:], item.Signature)
		for _, raw := range item.InputPath {
			var value [32]byte
			if !copy32(&value, raw) {
				return Proof{}, errors.New("claim input path is invalid")
			}
			claim.InputPath = append(claim.InputPath, value)
		}
		proof.Claims = append(proof.Claims, claim)
	}
	for _, raw := range wire.SignerIDs {
		var value [32]byte
		if !copy32(&value, raw) {
			return Proof{}, errors.New("claim signer id is invalid")
		}
		proof.SignerIDs = append(proof.SignerIDs, value)
	}
	proof.Signatures = wire.Signatures
	for _, alternate := range wire.AlternateSets {
		value, err := wireToProof(alternate)
		if err != nil {
			return Proof{}, err
		}
		proof.AlternateSets = append(proof.AlternateSets, value)
	}
	return proof, nil
}

func copy32(target *[32]byte, raw []byte) bool {
	if len(raw) != len(target) {
		return false
	}
	copy(target[:], raw)
	return true
}
