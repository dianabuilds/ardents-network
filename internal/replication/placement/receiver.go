package placement

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"sync"
	"time"

	identityprincipal "ardents/internal/identity/principal"
)

const (
	defaultReservationTTL = 2 * time.Minute
	defaultLeaseTTL       = 24 * time.Hour
)

type reservation struct {
	offer       ReservationOffer
	principal   identityprincipal.ID
	result      ReservationResult
	tokenDigest [32]byte
}

type Receiver struct {
	mu           sync.Mutex
	cfg          ReceiverConfig
	reserved     int64
	used         int64
	reservations map[string]reservation
	commitments  map[string]Commitment
	committing   map[string]bool
}

func NewReceiver(cfg ReceiverConfig) *Receiver {
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Random == nil {
		cfg.Random = rand.Reader
	}
	return &Receiver{
		cfg: cfg, reservations: map[string]reservation{}, commitments: map[string]Commitment{}, committing: map[string]bool{},
	}
}

func (r *Receiver) Capacity() Capacity {
	r.mu.Lock()
	defer r.mu.Unlock()
	free := max(r.cfg.MaxBytes-r.used-r.reserved, 0)
	return Capacity{
		NodePrincipal: r.cfg.NodePrincipal, FreeBytes: free, ReservedBytes: r.reserved,
		UsedBytes: r.used, ObservedAt: r.cfg.Now().UTC(),
	}
}

func (r *Receiver) Reserve(offer ReservationOffer, auth PeerAuthorization) (ReservationResult, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.cfg.Now().UTC()
	if existing, ok := r.reservations[offer.OperationID]; ok {
		if existing.offer != offer || !existing.principal.Equal(auth.NodePrincipal) {
			return ReservationResult{}, fmt.Errorf("reservation replay conflicts with existing operation")
		}
		return existing.result, nil
	}
	if err := validateOffer(offer, now); err != nil {
		return ReservationResult{}, err
	}
	if offer.ProtocolVersion != ReplicaProtocolVersion || offer.EncryptedSize > MaxInlineReplicaBytes {
		return r.rememberRejection(offer, auth.NodePrincipal, ReasonUnsupported), nil
	}
	if offer.RequestedLease <= 0 || offer.RequestedLease > defaultLeaseTTL {
		return r.rememberRejection(offer, auth.NodePrincipal, ReasonLease), nil
	}
	if reason := authorizationDenial(auth); reason != "" {
		return r.rememberRejection(offer, auth.NodePrincipal, reason), nil
	}
	if r.cfg.MaxBytes > 0 && r.used+r.reserved+offer.EncryptedSize > r.cfg.MaxBytes {
		return r.rememberRejection(offer, auth.NodePrincipal, ReasonQuota), nil
	}
	token, digest, err := newToken(r.cfg.Random)
	if err != nil {
		return ReservationResult{}, err
	}
	result := ReservationResult{OperationID: offer.OperationID, Status: ReservationAccepted, Token: token, ExpiresAt: offer.ExpiresAt.UTC()}
	r.reservations[offer.OperationID] = reservation{offer: offer, principal: auth.NodePrincipal, result: result, tokenDigest: digest}
	r.reserved += offer.EncryptedSize
	return result, nil
}

func (r *Receiver) rememberRejection(offer ReservationOffer, principal identityprincipal.ID, reason string) ReservationResult {
	result := ReservationResult{OperationID: offer.OperationID, Status: ReservationRejected, Reason: reason, ExpiresAt: offer.ExpiresAt.UTC()}
	r.reservations[offer.OperationID] = reservation{offer: offer, principal: principal, result: result}
	return result
}

func validateOffer(offer ReservationOffer, now time.Time) error {
	if offer.OperationID == "" || offer.ProtocolVersion == 0 || offer.IntentVersion == 0 || offer.BlobID == "" || offer.CID == "" || offer.BlobID != offer.CID || offer.Nonce == "" || offer.EncryptedSize <= 0 {
		return fmt.Errorf("reservation offer is incomplete")
	}
	if !offer.ExpiresAt.After(now) || offer.ExpiresAt.After(now.Add(defaultReservationTTL)) {
		return fmt.Errorf("reservation offer expiry is invalid")
	}
	return nil
}

func authorizationDenial(auth PeerAuthorization) string {
	switch {
	case auth.NodePrincipal.String() == "" || !auth.Authenticated || !auth.Trusted:
		return ReasonUntrusted
	case !auth.CapabilityValid:
		return ReasonCapability
	case !auth.PolicyAllowed:
		return ReasonPolicy
	default:
		return ""
	}
}

func newToken(randomReader io.Reader) (string, [32]byte, error) {
	var raw [24]byte
	if _, err := io.ReadFull(randomReader, raw[:]); err != nil {
		return "", [32]byte{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw[:])
	return token, sha256.Sum256([]byte(token)), nil
}

func tokenMatches(expected [32]byte, token string) bool {
	actual := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(expected[:], actual[:]) == 1
}
