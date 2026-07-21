package replication

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"ardents/internal/content"
	model "ardents/internal/content/catalog"
	identityprincipal "ardents/internal/identity/principal"
	"ardents/internal/replication/availability"
	"ardents/internal/replication/placement"
)

const (
	actionReserveOffer   = "reserve_offer"
	actionReserveResult  = "reserve_result"
	actionCommitRequest  = "commit_request"
	actionCommitResult   = "commit_result"
	actionCapacityQuery  = "capacity_query"
	actionCapacityResult = "capacity_result"
	actionHealthQuery    = "health_query"
	actionHealthResult   = "health_result"
)

type DataService interface {
	GetBlob(string) (model.Blob, bool)
	GetBlobPayload(string) ([]byte, error)
	ReserveReplica(placement.ReservationOffer, placement.PeerAuthorization) (placement.ReservationResult, error)
	CommitReplica(placement.CommitRequest, placement.PeerAuthorization) (placement.Commitment, error)
	ObserveReplicaCommitment(placement.Commitment, time.Time) (placement.Commitment, error)
	ReplicaPlacementState() placement.State
	RenewReplicaCommitment(string, time.Time, time.Time) (placement.Commitment, error)
	MarkReplicaCommitment(string, string, time.Time, string) (placement.Commitment, error)
	ListReplicaIntents() []availability.ReplicaIntent
	ReconcileAvailability(string, time.Time) (availability.ReconcileResult, error)
	RecordRepairFailure(string, time.Time, string) (availability.RepairRecord, error)
	ReplicaCapacity() placement.Capacity
}

type PolicyService interface {
	AllowBlobRetention(content.BlobPolicyView, bool, time.Time, time.Time) error
	AllowPeerBlobReserving(content.BlobPolicyView) error
}

type controlWire struct {
	Action      string          `json:"action"`
	OperationID string          `json:"operation_id"`
	Source      string          `json:"source"`
	Target      string          `json:"target"`
	PublicKey   string          `json:"public_key"`
	Body        json.RawMessage `json:"body"`
	Signature   string          `json:"signature"`
}

type reserveOfferBody struct {
	Offer placement.ReservationOffer `json:"offer"`
	Blob  model.Blob                 `json:"blob"`
}

type reserveResultBody struct {
	Result placement.ReservationResult `json:"result"`
}

type commitRequestBody struct {
	Request    placement.CommitRequest `json:"request"`
	Ciphertext string                  `json:"ciphertext"`
}

type commitResultBody struct {
	Commitment placement.Commitment `json:"commitment"`
	Status     string               `json:"status"`
	Reason     string               `json:"reason,omitempty"`
}

type capacityQueryBody struct {
	Blob model.Blob `json:"blob"`
}

type capacityResultBody struct {
	Capacity placement.Capacity `json:"capacity"`
	Status   string             `json:"status"`
	Reason   string             `json:"reason,omitempty"`
}

type healthQueryBody struct {
	Commitment              placement.Commitment `json:"commitment"`
	RequestedLeaseExpiresAt time.Time            `json:"requested_lease_expires_at"`
}

type healthResultBody struct {
	Commitment placement.Commitment `json:"commitment"`
	Status     string               `json:"status"`
	Reason     string               `json:"reason,omitempty"`
}

func signControl(action, operationID, source, target, publicKey string, body any, key ed25519.PrivateKey) ([]byte, error) {
	rawBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	wire := controlWire{Action: action, OperationID: operationID, Source: source, Target: target, PublicKey: publicKey, Body: rawBody}
	canonical, err := canonicalControl(wire)
	if err != nil {
		return nil, err
	}
	wire.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, canonical))
	return json.Marshal(wire)
}

func verifyControl(raw []byte, localNodeID string) (controlWire, error) {
	var wire controlWire
	if err := json.Unmarshal(raw, &wire); err != nil {
		return controlWire{}, err
	}
	if wire.Action == "" || wire.OperationID == "" || wire.Source == "" || wire.Target != localNodeID || len(wire.Body) == 0 {
		return controlWire{}, fmt.Errorf("replica control routing is invalid")
	}
	expected, err := identityprincipal.FromPublicKey(wire.PublicKey)
	if err != nil || expected != wire.Source {
		return controlWire{}, fmt.Errorf("replica control node identity is invalid")
	}
	publicKey, err := base64.StdEncoding.DecodeString(wire.PublicKey)
	if err != nil {
		return controlWire{}, fmt.Errorf("replica control public key is invalid")
	}
	signature, err := base64.StdEncoding.DecodeString(wire.Signature)
	if err != nil {
		return controlWire{}, fmt.Errorf("replica control signature is invalid")
	}
	canonical, err := canonicalControl(wire)
	if err != nil || !ed25519.Verify(publicKey, canonical, signature) {
		return controlWire{}, fmt.Errorf("replica control signature verification failed")
	}
	return wire, nil
}

func canonicalControl(wire controlWire) ([]byte, error) {
	return json.Marshal(struct {
		Action, OperationID, Source, Target, PublicKey string
		Body                                           json.RawMessage
	}{wire.Action, wire.OperationID, wire.Source, wire.Target, wire.PublicKey, wire.Body})
}
