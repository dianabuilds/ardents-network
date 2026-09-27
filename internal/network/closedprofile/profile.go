package closedprofile

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"
)

const (
	profileMagic   = "ARDCPR03"
	profileVersion = uint16(3)
	MaxSize        = 64 << 10
	MaxNodes       = 32
	MaxKeys        = 18
)

type Node struct {
	NodeID         [32]byte
	RecordDigest   [32]byte
	RoleDomain     byte
	Subrole        byte
	DutyGeneration uint64
}

type TokenKey struct {
	WindowStart uint64
	Class       byte
	SPKI        []byte
}

// Profile is the parsed, signed profile before State joins it to the
// authenticated current candidate view. It deliberately has no acceptance
// side effect; callers must finish that State-owned join before exposing it.
type Profile struct {
	NetworkID, StateGeneration, EpochDigest, IssuerNodeID, AuthorityKey [32]byte
	Epoch                                                               uint64
	NotBefore, NotAfter                                                 time.Time
	Nodes                                                               []Node
	Keys                                                                []TokenKey
	Digest                                                              [32]byte
}

// Context names the caller-supplied State Epoch and pinned authority. State
// supplies authenticated current facts before using Verify for acceptance.
type Context struct {
	StateGeneration, NetworkID, EpochDigest [32]byte
	Epoch                                   uint64
	Authority                               ed25519.PublicKey
	Now                                     time.Time
}

// Verify authenticates one exact signed profile against its supplied State context.
// The returned token SPKI bytes are owned copies; State must still join Node
// entries to its current verified candidate view before accepting the profile.
func Verify(raw []byte, context Context) (Profile, error) {
	if len(raw) == 0 || len(raw) > MaxSize || len(context.Authority) != ed25519.PublicKeySize {
		return Profile{}, errors.New("closed profile framing is invalid")
	}
	if len(raw) < len(profileMagic)+2+ed25519.SignatureSize {
		return Profile{}, errors.New("closed profile is truncated")
	}
	body, signature := raw[:len(raw)-ed25519.SignatureSize], raw[len(raw)-ed25519.SignatureSize:]
	if !ed25519.Verify(context.Authority, append([]byte("ardents-closed-profile-v3\x00"), body...), signature) {
		return Profile{}, errors.New("closed profile signature is invalid")
	}
	profile, err := parseBody(body, context.StateGeneration, context.NetworkID, context.EpochDigest, context.Epoch, context.Now.UTC())
	if err != nil {
		return Profile{}, err
	}
	profile.Digest = sha256.Sum256(raw)
	return profile, nil
}

func parseBody(body []byte, stateGeneration, networkID, epochDigest [32]byte, epoch uint64, now time.Time) (Profile, error) {
	d := newDecoder(body)
	magic, err := d.bytes(len(profileMagic))
	if err != nil || string(magic) != profileMagic {
		return Profile{}, errors.New("closed profile magic is invalid")
	}
	version, err := d.uint16()
	if err != nil || version != profileVersion {
		return Profile{}, errors.New("closed profile version is invalid")
	}
	profile := Profile{}
	for _, field := range []*[32]byte{&profile.NetworkID, &profile.StateGeneration} {
		value, readErr := d.bytes(32)
		if readErr != nil {
			return Profile{}, errors.New("closed profile identity is invalid")
		}
		copy(field[:], value)
	}
	if profile.Epoch, err = d.uint64(); err != nil || profile.Epoch == 0 {
		return Profile{}, errors.New("closed profile epoch is invalid")
	}
	value, err := d.bytes(32)
	if err != nil {
		return Profile{}, errors.New("closed profile epoch digest is invalid")
	}
	copy(profile.EpochDigest[:], value)
	from, err := d.uint64()
	if err != nil {
		return Profile{}, errors.New("closed profile validity is invalid")
	}
	until, err := d.uint64()
	if err != nil || until <= from || until-from > uint64((6*time.Hour).Seconds()) {
		return Profile{}, errors.New("closed profile validity is invalid")
	}
	profile.NotBefore, profile.NotAfter = time.Unix(int64(from), 0).UTC(), time.Unix(int64(until), 0).UTC()
	for _, field := range []*[32]byte{&profile.IssuerNodeID, &profile.AuthorityKey} {
		value, readErr := d.bytes(32)
		if readErr != nil {
			return Profile{}, errors.New("closed profile authority is invalid")
		}
		copy(field[:], value)
	}
	if profile.NetworkID != networkID || profile.StateGeneration != stateGeneration || profile.Epoch != epoch || profile.EpochDigest != epochDigest ||
		profile.AuthorityKey == [32]byte{} || now.Before(profile.NotBefore) || !now.Before(profile.NotAfter) {
		return Profile{}, errors.New("closed profile does not match current state")
	}
	if err := decodeNodes(&d, &profile); err != nil {
		return Profile{}, err
	}
	if err := decodeKeys(&d, &profile); err != nil || !d.done() {
		return Profile{}, errors.New("closed profile keys are invalid")
	}
	return profile, nil
}

func decodeNodes(d *decoder, profile *Profile) error {
	count, err := d.uint16()
	if err != nil || count == 0 || count > MaxNodes {
		return errors.New("closed profile node count is invalid")
	}
	issuerCount := 0
	for range int(count) {
		var node Node
		for _, field := range []*[32]byte{&node.NodeID, &node.RecordDigest} {
			value, readErr := d.bytes(32)
			if readErr != nil {
				return errors.New("closed profile node is invalid")
			}
			copy(field[:], value)
		}
		if node.RoleDomain, err = d.byte(); err != nil {
			return errors.New("closed profile node is invalid")
		}
		if node.Subrole, err = d.byte(); err != nil {
			return errors.New("closed profile node is invalid")
		}
		if node.DutyGeneration, err = d.uint64(); err != nil || node.DutyGeneration == 0 || !validDuty(node.RoleDomain, node.Subrole) || (len(profile.Nodes) > 0 && bytes.Compare(profile.Nodes[len(profile.Nodes)-1].NodeID[:], node.NodeID[:]) >= 0) {
			return errors.New("closed profile nodes are not canonical")
		}
		if node.Subrole == 6 {
			issuerCount++
			if node.NodeID != profile.IssuerNodeID {
				return errors.New("closed profile issuer does not match its duty")
			}
		}
		profile.Nodes = append(profile.Nodes, node)
	}
	if issuerCount != 1 {
		return errors.New("closed profile has no unique issuer duty")
	}
	return nil
}

func validDuty(domain, subrole byte) bool {
	switch subrole {
	case 1, 2:
		return domain == 1 || domain == 3 || domain == 4
	case 3:
		return domain == 4
	case 4, 5, 6:
		return domain == 2
	default:
		return false
	}
}

func decodeKeys(d *decoder, profile *Profile) error {
	count, err := d.uint16()
	if err != nil || count == 0 || count > MaxKeys {
		return errors.New("closed profile key count is invalid")
	}
	seen := make(map[[32]byte]bool, count)
	for range int(count) {
		key := TokenKey{}
		if key.WindowStart, err = d.uint64(); err != nil || key.WindowStart%uint64(time.Hour.Seconds()) != 0 {
			return errors.New("closed profile key window is invalid")
		}
		windowStart := time.Unix(int64(key.WindowStart), 0).UTC()
		if windowStart.Before(profile.NotBefore) || windowStart.Add(time.Hour).After(profile.NotAfter) {
			return errors.New("closed profile key window is outside its validity")
		}
		if key.Class, err = d.byte(); err != nil || key.Class < 1 || key.Class > 3 {
			return errors.New("closed profile key class is invalid")
		}
		length, readErr := d.uint16()
		if readErr != nil || length != 346 {
			return errors.New("closed profile key length is invalid")
		}
		key.SPKI, err = d.bytes(int(length))
		if err != nil || (len(profile.Keys) > 0 && (profile.Keys[len(profile.Keys)-1].WindowStart > key.WindowStart || profile.Keys[len(profile.Keys)-1].WindowStart == key.WindowStart && profile.Keys[len(profile.Keys)-1].Class >= key.Class)) {
			return errors.New("closed profile keys are not canonical")
		}
		keyID := sha256.Sum256(key.SPKI)
		if !ValidateTokenSPKI(key.SPKI) || seen[keyID] {
			return errors.New("closed profile reuses a token key")
		}
		seen[keyID] = true
		key.SPKI = append([]byte(nil), key.SPKI...)
		profile.Keys = append(profile.Keys, key)
	}
	return nil
}
