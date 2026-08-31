package endpoint

import (
	"bytes"
	"encoding/json"

	"github.com/dianabuilds/ardents-network/internal/route"
)

// transitAcquisitionStateV1 is the accepted persisted Introduction-only
// record. New commits always use v2; this shape exists only to migrate an exact
// previously accepted root without reviving the implicit-role request grammar.
type transitAcquisitionStateV1 struct {
	Schema                         string                  `json:"schema"`
	Phase                          transitAcquisitionPhase `json:"phase"`
	NetworkID, Digest              [32]byte
	Epoch                          uint64
	IssuerNodeID, IssuerPublicKey  [32]byte
	IssuerProfileDigest            [32]byte
	GrantSignerPublicKey           [32]byte
	IntroductionNodeID             [32]byte
	NotAfter                       int64
	RequestID, AttachmentID        [32]byte
	ClientKeyDigest                [32]byte
	Certificate, PrivateKey, Grant []byte
}

func decodeTransitAcquisitionState(raw []byte) (transitAcquisitionState, error) {
	var header struct {
		Schema string `json:"schema"`
	}
	if err := json.Unmarshal(raw, &header); err != nil {
		return transitAcquisitionState{}, err
	}
	if header.Schema == transitAcquisitionSchemaV1 {
		var previous transitAcquisitionStateV1
		decoder := json.NewDecoder(bytes.NewReader(raw))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&previous); err != nil {
			return transitAcquisitionState{}, err
		}
		state := transitAcquisitionState{Schema: transitAcquisitionSchema, Phase: previous.Phase,
			NetworkID: previous.NetworkID, Digest: previous.Digest, Epoch: previous.Epoch,
			IssuerNodeID: previous.IssuerNodeID, IssuerPublicKey: previous.IssuerPublicKey,
			IssuerProfileDigest: previous.IssuerProfileDigest, GrantSignerPublicKey: previous.GrantSignerPublicKey,
			TransitNodeID: previous.IntroductionNodeID, TransitRole: route.IntroductionRole,
			NotAfter: previous.NotAfter, RequestID: previous.RequestID, AttachmentID: previous.AttachmentID,
			ClientKeyDigest: previous.ClientKeyDigest, Certificate: previous.Certificate,
			PrivateKey: previous.PrivateKey, Grant: previous.Grant}
		if !validTransitAcquisitionState(state) {
			return transitAcquisitionState{}, errInvalidTransitAcquisitionState
		}
		return state, nil
	}
	var state transitAcquisitionState
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&state); err != nil || !validTransitAcquisitionState(state) {
		return transitAcquisitionState{}, errInvalidTransitAcquisitionState
	}
	return state, nil
}
