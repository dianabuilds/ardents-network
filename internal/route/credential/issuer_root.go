package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"time"

	"github.com/cloudflare/circl/hpke"
	"github.com/openpcc/ohttp"
)

const issuerMaterialVersion = byte(1)

// InitializeIssuerRoot creates or reopens one immutable purpose-scoped issuer
// generation and returns only its Node-signed public profile.
func InitializeIssuerRoot(config IssuerRootConfig) (IssuerRootReceipt, error) {
	now := time.Time{}
	if config.Clock != nil {
		now = config.Clock().UTC()
	}
	if config.Root == "" || config.NetworkID == [32]byte{} || config.NodeID == [32]byte{} ||
		len(config.IdentityKey) != ed25519.PrivateKeySize || config.InitiatorNodeID == [32]byte{} ||
		config.InitiatorPublicKey == [32]byte{} || config.Budget == 0 || now.IsZero() ||
		!config.AssignmentNotAfter.Equal(config.AssignmentNotAfter.UTC().Truncate(time.Second)) || !now.Before(config.AssignmentNotAfter) {
		return IssuerRootReceipt{}, errors.New("transit grant issuer root configuration is invalid")
	}
	grantPublic, grantPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	kem := hpke.KEM_P256_HKDF_SHA256
	_, ohttpPrivate, err := kem.Scheme().GenerateKeyPair()
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	ohttpMaterial, err := ohttpPrivate.MarshalBinary()
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	privateMaterial := encodeIssuerMaterial(grantPrivate, ohttpMaterial)
	profile, err := issuerProfile(config, grantPublic, ohttpMaterial)
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	raw, err := EncodeProfile(profile)
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	ledger, err := openIssuerRootStore(config.Root, config.Clock, true)
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	retainedProfile, initializeErr := ledger.initialize(sha256.Sum256(raw), config.Budget, privateMaterial, raw)
	retainedPrivate, _, materialErr := ledger.material()
	closeErr := ledger.close()
	if initializeErr != nil {
		return IssuerRootReceipt{}, errors.Join(initializeErr, closeErr)
	}
	if materialErr != nil {
		return IssuerRootReceipt{}, errors.Join(materialErr, closeErr)
	}
	if closeErr != nil {
		return IssuerRootReceipt{}, closeErr
	}
	retainedGrant, retainedOHTTP, err := decodeIssuerMaterial(retainedPrivate)
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	retained, err := issuerProfile(config, retainedGrant.Public().(ed25519.PublicKey), retainedOHTTP)
	if err != nil {
		return IssuerRootReceipt{}, err
	}
	verified, err := EncodeProfile(retained)
	if err != nil || !bytes.Equal(verified, retainedProfile) {
		return IssuerRootReceipt{}, errors.New("transit grant issuer root does not match the requested public binding")
	}
	return IssuerRootReceipt{Profile: append([]byte(nil), retainedProfile...), ProfileDigest: sha256.Sum256(retainedProfile)}, nil
}

func issuerProfile(config IssuerRootConfig, grantPublic ed25519.PublicKey, ohttpMaterial []byte) (Profile, error) {
	secret, err := hpke.KEM_P256_HKDF_SHA256.Scheme().UnmarshalBinaryPrivateKey(ohttpMaterial)
	if err != nil || len(grantPublic) != ed25519.PublicKeySize {
		return Profile{}, errors.New("transit grant issuer private material is invalid")
	}
	keyConfig := ohttp.KeyConfig{KeyID: 1, KemID: hpke.KEM_P256_HKDF_SHA256, PublicKey: secret.Public(),
		SymmetricAlgorithms: []ohttp.SymmetricAlgorithm{{KDFID: hpke.KDF_HKDF_SHA256, AEADID: hpke.AEAD_AES128GCM}}}
	encoded, err := keyConfig.MarshalBinary()
	if err != nil {
		return Profile{}, err
	}
	profile := Profile{Version: profileVersion, NetworkID: config.NetworkID, NodeID: config.NodeID,
		GrantSignerID: sha256.Sum256(grantPublic), GrantSignerPublicKey: publicIdentifierFromBytes(grantPublic),
		InitiatorNodeID: config.InitiatorNodeID, InitiatorPublicKey: config.InitiatorPublicKey,
		KeyConfig: encoded, KeyConfigDigest: sha256.Sum256(encoded), AssignmentNotAfter: config.AssignmentNotAfter.UTC()}
	profile.Signature = ed25519.Sign(config.IdentityKey, profileTranscript(profile))
	return profile, nil
}

func encodeIssuerMaterial(grantPrivate ed25519.PrivateKey, ohttpMaterial []byte) []byte {
	out := make([]byte, 0, 5+ed25519.PrivateKeySize+2+len(ohttpMaterial))
	out = append(out, "ATIR"...)
	out = append(out, issuerMaterialVersion)
	out = append(out, grantPrivate...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(ohttpMaterial)))
	return append(out, ohttpMaterial...)
}

func decodeIssuerMaterial(raw []byte) (ed25519.PrivateKey, []byte, error) {
	const fixed = 4 + 1 + ed25519.PrivateKeySize + 2
	if len(raw) <= fixed || string(raw[:4]) != "ATIR" || raw[4] != issuerMaterialVersion {
		return nil, nil, errors.New("transit grant issuer private material is invalid")
	}
	offset := 5
	grantPrivate := append(ed25519.PrivateKey(nil), raw[offset:offset+ed25519.PrivateKeySize]...)
	offset += ed25519.PrivateKeySize
	length := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if length == 0 || offset+length != len(raw) {
		return nil, nil, errors.New("transit grant issuer private material is invalid")
	}
	return grantPrivate, append([]byte(nil), raw[offset:]...), nil
}

func publicIdentifierFromBytes(public ed25519.PublicKey) [32]byte {
	var result [32]byte
	copy(result[:], public)
	return result
}
