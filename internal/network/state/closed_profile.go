package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"errors"
	"time"
)

var (
	closedProfileMGF1   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	closedProfileRSAPSS = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	closedProfileSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	closedProfileNull   = []byte{0x05, 0x00}
)

const (
	closedProfileMagic        = "ARDCPR03"
	closedProfileVersion      = uint16(3)
	maximumClosedProfileSize  = 64 << 10
	maximumClosedProfileNodes = 32
	maximumClosedProfileKeys  = 18
)

type closedProfileNode struct {
	nodeID       [32]byte
	recordDigest [32]byte
	domain       byte
	subrole      byte
	generation   uint64
}

type closedProfileKey struct {
	windowStart uint64
	class       byte
	spki        []byte
}

type closedProfileAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type closedProfilePSSParameters struct {
	HashAlgorithm    closedProfileAlgorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm closedProfileAlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                              `asn1:"explicit,tag:2"`
}

type closedProfileSubjectPublicKeyInfo struct {
	Algorithm        closedProfileAlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

// closedProfile is the parsed, signed profile before State joins it to the
// authenticated current candidate view. It deliberately has no acceptance
// side effect; callers must finish that State-owned join before exposing it.
type closedProfile struct {
	networkID, stateGeneration, epochDigest, issuerNodeID, authorityKey [32]byte
	epoch                                                               uint64
	notBefore, notAfter                                                 time.Time
	nodes                                                               []closedProfileNode
	keys                                                                []closedProfileKey
	digest                                                              [32]byte
}

func parseClosedProfile(raw []byte, stateGeneration, networkID, epochDigest [32]byte, epoch uint64, authority ed25519.PublicKey, now time.Time) (closedProfile, error) {
	if len(raw) == 0 || len(raw) > maximumClosedProfileSize || len(authority) != ed25519.PublicKeySize {
		return closedProfile{}, errors.New("closed profile framing is invalid")
	}
	if len(raw) < len(closedProfileMagic)+2+ed25519.SignatureSize {
		return closedProfile{}, errors.New("closed profile is truncated")
	}
	body, signature := raw[:len(raw)-ed25519.SignatureSize], raw[len(raw)-ed25519.SignatureSize:]
	if !ed25519.Verify(authority, append([]byte("ardents-closed-profile-v3\x00"), body...), signature) {
		return closedProfile{}, errors.New("closed profile signature is invalid")
	}
	d := newDecoder(body)
	magic, err := d.bytes(len(closedProfileMagic))
	if err != nil || string(magic) != closedProfileMagic {
		return closedProfile{}, errors.New("closed profile magic is invalid")
	}
	version, err := d.uint16()
	if err != nil || version != closedProfileVersion {
		return closedProfile{}, errors.New("closed profile version is invalid")
	}
	profile := closedProfile{}
	for _, field := range []*[32]byte{&profile.networkID, &profile.stateGeneration} {
		value, readErr := d.bytes(32)
		if readErr != nil {
			return closedProfile{}, errors.New("closed profile identity is invalid")
		}
		copy(field[:], value)
	}
	if profile.epoch, err = d.uint64(); err != nil || profile.epoch == 0 {
		return closedProfile{}, errors.New("closed profile epoch is invalid")
	}
	value, err := d.bytes(32)
	if err != nil {
		return closedProfile{}, errors.New("closed profile epoch digest is invalid")
	}
	copy(profile.epochDigest[:], value)
	from, err := d.uint64()
	if err != nil {
		return closedProfile{}, errors.New("closed profile validity is invalid")
	}
	until, err := d.uint64()
	if err != nil || until <= from || until-from > uint64((6*time.Hour).Seconds()) {
		return closedProfile{}, errors.New("closed profile validity is invalid")
	}
	profile.notBefore, profile.notAfter = time.Unix(int64(from), 0).UTC(), time.Unix(int64(until), 0).UTC()
	for _, field := range []*[32]byte{&profile.issuerNodeID, &profile.authorityKey} {
		value, readErr := d.bytes(32)
		if readErr != nil {
			return closedProfile{}, errors.New("closed profile authority is invalid")
		}
		copy(field[:], value)
	}
	if profile.networkID != networkID || profile.stateGeneration != stateGeneration || profile.epoch != epoch || profile.epochDigest != epochDigest ||
		profile.authorityKey == [32]byte{} || now.Before(profile.notBefore) || !now.Before(profile.notAfter) {
		return closedProfile{}, errors.New("closed profile does not match current state")
	}
	if err := decodeClosedProfileNodes(&d, &profile); err != nil {
		return closedProfile{}, err
	}
	if err := decodeClosedProfileKeys(&d, &profile); err != nil || !d.done() {
		return closedProfile{}, errors.New("closed profile keys are invalid")
	}
	profile.digest = sha256.Sum256(raw)
	return profile, nil
}

func decodeClosedProfileNodes(d *decoder, profile *closedProfile) error {
	count, err := d.uint16()
	if err != nil || count == 0 || count > maximumClosedProfileNodes {
		return errors.New("closed profile node count is invalid")
	}
	issuerCount := 0
	for range int(count) {
		var node closedProfileNode
		for _, field := range []*[32]byte{&node.nodeID, &node.recordDigest} {
			value, readErr := d.bytes(32)
			if readErr != nil {
				return errors.New("closed profile node is invalid")
			}
			copy(field[:], value)
		}
		if node.domain, err = d.byte(); err != nil {
			return errors.New("closed profile node is invalid")
		}
		if node.subrole, err = d.byte(); err != nil {
			return errors.New("closed profile node is invalid")
		}
		if node.generation, err = d.uint64(); err != nil || node.generation == 0 || !validClosedProfileDuty(node.domain, node.subrole) || (len(profile.nodes) > 0 && bytes.Compare(profile.nodes[len(profile.nodes)-1].nodeID[:], node.nodeID[:]) >= 0) {
			return errors.New("closed profile nodes are not canonical")
		}
		if node.subrole == 6 {
			issuerCount++
			if node.nodeID != profile.issuerNodeID {
				return errors.New("closed profile issuer does not match its duty")
			}
		}
		profile.nodes = append(profile.nodes, node)
	}
	if issuerCount != 1 {
		return errors.New("closed profile has no unique issuer duty")
	}
	return nil
}

func validClosedProfileDuty(domain, subrole byte) bool {
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

func decodeClosedProfileKeys(d *decoder, profile *closedProfile) error {
	count, err := d.uint16()
	if err != nil || count == 0 || count > maximumClosedProfileKeys {
		return errors.New("closed profile key count is invalid")
	}
	seen := make(map[[32]byte]bool, count)
	for range int(count) {
		key := closedProfileKey{}
		if key.windowStart, err = d.uint64(); err != nil || key.windowStart%uint64(time.Hour.Seconds()) != 0 {
			return errors.New("closed profile key window is invalid")
		}
		windowStart := time.Unix(int64(key.windowStart), 0).UTC()
		if windowStart.Before(profile.notBefore) || windowStart.Add(time.Hour).After(profile.notAfter) {
			return errors.New("closed profile key window is outside its validity")
		}
		if key.class, err = d.byte(); err != nil || key.class < 1 || key.class > 3 {
			return errors.New("closed profile key class is invalid")
		}
		length, readErr := d.uint16()
		if readErr != nil || length != 346 {
			return errors.New("closed profile key length is invalid")
		}
		key.spki, err = d.bytes(int(length))
		if err != nil || (len(profile.keys) > 0 && (profile.keys[len(profile.keys)-1].windowStart > key.windowStart || profile.keys[len(profile.keys)-1].windowStart == key.windowStart && profile.keys[len(profile.keys)-1].class >= key.class)) {
			return errors.New("closed profile keys are not canonical")
		}
		keyID := sha256.Sum256(key.spki)
		if !validClosedProfileSPKI(key.spki) || seen[keyID] {
			return errors.New("closed profile reuses a token key")
		}
		seen[keyID] = true
		profile.keys = append(profile.keys, key)
	}
	return nil
}

func validClosedProfileSPKI(encoded []byte) bool {
	var subjectPublicKeyInfo closedProfileSubjectPublicKeyInfo
	rest, err := asn1.Unmarshal(encoded, &subjectPublicKeyInfo)
	if err != nil || len(rest) != 0 || !subjectPublicKeyInfo.Algorithm.Algorithm.Equal(closedProfileRSAPSS) || subjectPublicKeyInfo.SubjectPublicKey.BitLength%8 != 0 {
		return false
	}
	var parameters closedProfilePSSParameters
	rest, err = asn1.Unmarshal(subjectPublicKeyInfo.Algorithm.Parameters.FullBytes, &parameters)
	if err != nil || len(rest) != 0 || !parameters.HashAlgorithm.Algorithm.Equal(closedProfileSHA384) ||
		!bytes.Equal(parameters.HashAlgorithm.Parameters.FullBytes, closedProfileNull) || !parameters.MaskGenAlgorithm.Algorithm.Equal(closedProfileMGF1) || parameters.SaltLength != 48 {
		return false
	}
	var mgfHash closedProfileAlgorithmIdentifier
	rest, err = asn1.Unmarshal(parameters.MaskGenAlgorithm.Parameters.FullBytes, &mgfHash)
	if err != nil || len(rest) != 0 || !mgfHash.Algorithm.Equal(closedProfileSHA384) || !bytes.Equal(mgfHash.Parameters.FullBytes, closedProfileNull) {
		return false
	}
	publicKey, err := x509.ParsePKCS1PublicKey(subjectPublicKeyInfo.SubjectPublicKey.Bytes)
	return err == nil && publicKey.N.BitLen() == 2048 && publicKey.E == 65537 && publicKey.N.Sign() > 0 && publicKey.N.Bit(0) == 1 && (publicKey.N.BitLen()+7)/8 == 256
}

// ValidateClosedTokenSPKI reports whether encoded is the one selected
// byte-exact RSA-PSS SubjectPublicKeyInfo grammar for a closed profile token
// key. It does not accept an alternative generic RSA SPKI representation.
func ValidateClosedTokenSPKI(encoded []byte) bool {
	return validClosedProfileSPKI(encoded)
}
