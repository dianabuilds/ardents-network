package credential

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/dianabuilds/ardents-network/internal/network/state"
)

const (
	closedIssuerRootMarkerName  = ".ardents-closed-issuer-v1"
	closedIssuerRootMarker      = "ardents-closed-issuer-v1\n"
	closedIssuerMaterialName    = "closed-issuer-keys"
	closedIssuerProfileMagic    = "ARDCIP01"
	closedIssuerMaterialMagic   = "ARDCKR01"
	maximumClosedIssuerKeys     = 18
	maximumClosedIssuerMaterial = 64 << 10
)

var (
	closedIssuerMGF1   = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}
	closedIssuerRSAPSS = asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}
	closedIssuerSHA384 = asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}
	closedIssuerNull   = []byte{0x05, 0x00}
	closedIssuerDomain = []byte("ardents-closed-issuer-keys-v1\x00")
)

type closedIssuerAlgorithmIdentifier struct {
	Algorithm  asn1.ObjectIdentifier
	Parameters asn1.RawValue
}

type closedIssuerPSSParameters struct {
	HashAlgorithm    closedIssuerAlgorithmIdentifier `asn1:"explicit,tag:0"`
	MaskGenAlgorithm closedIssuerAlgorithmIdentifier `asn1:"explicit,tag:1"`
	SaltLength       int                             `asn1:"explicit,tag:2"`
}

type closedIssuerSPKI struct {
	Algorithm        closedIssuerAlgorithmIdentifier
	SubjectPublicKey asn1.BitString
}

type closedIssuerMaterial struct {
	network, node, signer [32]byte
	notBefore, notAfter   time.Time
	keys                  []closedIssuerPrivateKey
}

type closedIssuerPrivateKey struct {
	window time.Time
	class  byte
	der    []byte
}

// InitializeClosedIssuerRoot creates or reopens a finite immutable key root.
// It never imports another private key and returns only the Node-signed public
// SPKI inventory needed by State's closed-profile preparation.
func InitializeClosedIssuerRoot(config ClosedIssuerRootConfig) (ClosedIssuerRootReceipt, error) {
	if err := validateClosedIssuerRootConfig(config); err != nil {
		return ClosedIssuerRootReceipt{}, err
	}
	root, lease, err := openClosedIssuerRoot(config.Root)
	if err != nil {
		return ClosedIssuerRootReceipt{}, err
	}
	defer lease.release()
	materialPath := filepath.Join(root, closedIssuerMaterialName)
	var material closedIssuerMaterial
	if raw, readErr := readIssuerFile(materialPath, maximumClosedIssuerMaterial); readErr == nil {
		material, err = decodeClosedIssuerMaterial(raw)
		if err != nil {
			return ClosedIssuerRootReceipt{}, err
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return ClosedIssuerRootReceipt{}, readErr
	} else {
		material, err = generateClosedIssuerMaterial(config)
		if err != nil {
			return ClosedIssuerRootReceipt{}, err
		}
		raw, encodeErr := encodeClosedIssuerMaterial(material)
		if encodeErr != nil {
			return ClosedIssuerRootReceipt{}, encodeErr
		}
		defer clear(raw)
		if err := writeIssuerExclusive(materialPath, raw); err != nil {
			return ClosedIssuerRootReceipt{}, err
		}
		if err := syncIssuerDirectory(root); err != nil {
			return ClosedIssuerRootReceipt{}, err
		}
		persisted, reopenErr := readIssuerFile(materialPath, maximumClosedIssuerMaterial)
		if reopenErr != nil || !bytes.Equal(persisted, raw) {
			return ClosedIssuerRootReceipt{}, errors.New("closed issuer key root did not durably reopen")
		}
		clear(persisted)
	}
	if material.network != config.NetworkID || material.node != config.NodeID || material.signer != [32]byte(config.IdentityKey.Public().(ed25519.PublicKey)) ||
		!material.notBefore.Equal(config.NotBefore.UTC()) || !material.notAfter.Equal(config.NotAfter.UTC()) {
		return ClosedIssuerRootReceipt{}, errors.New("closed issuer key root cannot be replaced")
	}
	keys, err := closedIssuerPublicKeys(material)
	if err != nil {
		return ClosedIssuerRootReceipt{}, err
	}
	profile := ClosedIssuerProfile{NetworkID: material.network, NodeID: material.node, NotBefore: material.notBefore,
		NotAfter: material.notAfter, Keys: keys}
	raw, err := encodeClosedIssuerProfile(profile, config.IdentityKey)
	if err != nil {
		return ClosedIssuerRootReceipt{}, err
	}
	return ClosedIssuerRootReceipt{Profile: raw, ProfileDigest: sha256.Sum256(raw)}, nil
}

func validateClosedIssuerRootConfig(config ClosedIssuerRootConfig) error {
	now := time.Time{}
	if config.Clock != nil {
		now = config.Clock().UTC()
	}
	if config.Root == "" || config.NetworkID == [32]byte{} || config.NodeID == [32]byte{} || len(config.IdentityKey) != ed25519.PrivateKeySize ||
		now.IsZero() || config.NotBefore.IsZero() || config.NotBefore != config.NotBefore.UTC() || config.NotBefore.Truncate(time.Hour) != config.NotBefore ||
		!config.NotAfter.Equal(config.NotBefore.Add(1*time.Hour)) && !config.NotAfter.Equal(config.NotBefore.Add(2*time.Hour)) &&
			!config.NotAfter.Equal(config.NotBefore.Add(3*time.Hour)) && !config.NotAfter.Equal(config.NotBefore.Add(4*time.Hour)) &&
			!config.NotAfter.Equal(config.NotBefore.Add(5*time.Hour)) && !config.NotAfter.Equal(config.NotBefore.Add(6*time.Hour)) || !now.Before(config.NotAfter) {
		return errors.New("closed issuer key root configuration is invalid")
	}
	return nil
}

func openClosedIssuerRoot(root string) (string, issuerRootLease, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return "", issuerRootLease{}, err
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return "", issuerRootLease{}, err
	}
	info, err := os.Lstat(absolute)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", issuerRootLease{}, errors.New("closed issuer key root is not an owned directory")
	}
	lease, err := acquireIssuerRootLease(absolute)
	if err != nil {
		return "", issuerRootLease{}, err
	}
	fail := func(err error) (string, issuerRootLease, error) {
		_ = lease.release()
		return "", issuerRootLease{}, err
	}
	if err := validateIssuerRootPermissions(absolute, info); err != nil {
		return fail(err)
	}
	entries, err := os.ReadDir(absolute)
	if err != nil || len(entries) > 6 {
		return fail(errors.New("closed issuer key root entries are invalid"))
	}
	markerPath := filepath.Join(absolute, closedIssuerRootMarkerName)
	marker, markerErr := readIssuerFile(markerPath, int64(len(closedIssuerRootMarker)))
	if errors.Is(markerErr, os.ErrNotExist) {
		if len(entries) != 1 || entries[0].Name() != issuerRootLockName {
			return fail(errors.New("refusing to claim a non-fresh closed issuer key root"))
		}
		if err := writeIssuerExclusive(markerPath, []byte(closedIssuerRootMarker)); err != nil || syncIssuerDirectory(absolute) != nil {
			return fail(errors.New("initialize closed issuer key root marker"))
		}
	} else if markerErr != nil || !bytes.Equal(marker, []byte(closedIssuerRootMarker)) {
		return fail(errors.New("closed issuer key root marker is invalid"))
	}
	entries, err = os.ReadDir(absolute)
	if err != nil {
		return fail(err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() != issuerRootLockName && entry.Name() != closedIssuerRootMarkerName && entry.Name() != closedIssuerMaterialName && entry.Name() != closedTokenIssuerLedgerName && entry.Name() != closedIssuerLedgerBindingName && entry.Name() != closedIssuerLedgerStageName {
			return fail(errors.New("closed issuer key root has an unknown entry"))
		}
	}
	return absolute, lease, nil
}

func generateClosedIssuerMaterial(config ClosedIssuerRootConfig) (closedIssuerMaterial, error) {
	material := closedIssuerMaterial{network: config.NetworkID, node: config.NodeID, notBefore: config.NotBefore.UTC(), notAfter: config.NotAfter.UTC()}
	copy(material.signer[:], config.IdentityKey.Public().(ed25519.PublicKey))
	for window := material.notBefore; window.Before(material.notAfter); window = window.Add(time.Hour) {
		for class := byte(1); class <= 3; class++ {
			key, err := rsa.GenerateKey(rand.Reader, 2048)
			if err != nil || key.Validate() != nil || key.PublicKey.E != 65537 || key.N.BitLen() != 2048 {
				return closedIssuerMaterial{}, errors.New("generate closed issuer RSA key")
			}
			der := x509.MarshalPKCS1PrivateKey(key)
			material.keys = append(material.keys, closedIssuerPrivateKey{window: window, class: class, der: der})
		}
	}
	if _, err := closedIssuerPublicKeys(material); err != nil {
		return closedIssuerMaterial{}, err
	}
	return material, nil
}

func closedIssuerPublicKeys(material closedIssuerMaterial) ([]ClosedIssuerKey, error) {
	expected := closedIssuerKeyCount(material.notBefore, material.notAfter)
	if expected == 0 || expected > maximumClosedIssuerKeys || len(material.keys) != expected {
		return nil, errors.New("closed issuer key inventory is invalid")
	}
	keys := make([]ClosedIssuerKey, 0, len(material.keys))
	for index, item := range material.keys {
		if item.window.Before(material.notBefore) || !item.window.Before(material.notAfter) || item.window.Truncate(time.Hour) != item.window ||
			item.class < 1 || item.class > 3 || index > 0 && (material.keys[index-1].window.After(item.window) ||
			material.keys[index-1].window.Equal(item.window) && material.keys[index-1].class >= item.class) {
			return nil, errors.New("closed issuer key inventory is not canonical")
		}
		private, err := x509.ParsePKCS1PrivateKey(item.der)
		if err != nil || private.Validate() != nil || private.PublicKey.N.BitLen() != 2048 || private.PublicKey.E != 65537 {
			return nil, errors.New("closed issuer private key is invalid")
		}
		spki, err := encodeClosedIssuerSPKI(&private.PublicKey)
		if err != nil || !state.ValidateClosedTokenSPKI(spki) {
			return nil, errors.New("closed issuer public key is invalid")
		}
		keys = append(keys, ClosedIssuerKey{WindowStart: item.window, Class: item.class, SPKI: spki, KeyID: sha256.Sum256(spki)})
	}
	return keys, nil
}

func closedIssuerKeyCount(notBefore, notAfter time.Time) int {
	if notBefore.IsZero() || notAfter.IsZero() || notBefore.Truncate(time.Hour) != notBefore ||
		notAfter.Truncate(time.Hour) != notAfter || !notAfter.After(notBefore) || notAfter.After(notBefore.Add(6*time.Hour)) {
		return 0
	}
	return int(notAfter.Sub(notBefore)/time.Hour) * 3
}

func encodeClosedIssuerSPKI(public *rsa.PublicKey) ([]byte, error) {
	if public == nil || public.N == nil || public.N.BitLen() != 2048 || public.E != 65537 {
		return nil, errors.New("closed issuer RSA public key is invalid")
	}
	hash := closedIssuerAlgorithmIdentifier{Algorithm: closedIssuerSHA384, Parameters: asn1.RawValue{FullBytes: closedIssuerNull}}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		return nil, err
	}
	parameters, err := asn1.Marshal(closedIssuerPSSParameters{HashAlgorithm: hash,
		MaskGenAlgorithm: closedIssuerAlgorithmIdentifier{Algorithm: closedIssuerMGF1, Parameters: asn1.RawValue{FullBytes: hashDER}}, SaltLength: 48})
	if err != nil {
		return nil, err
	}
	rsaDER, err := asn1.Marshal(*public)
	if err != nil {
		return nil, err
	}
	return asn1.Marshal(closedIssuerSPKI{Algorithm: closedIssuerAlgorithmIdentifier{Algorithm: closedIssuerRSAPSS,
		Parameters: asn1.RawValue{FullBytes: parameters}}, SubjectPublicKey: asn1.BitString{Bytes: rsaDER, BitLength: len(rsaDER) * 8}})
}

func encodeClosedIssuerProfile(profile ClosedIssuerProfile, signer ed25519.PrivateKey) ([]byte, error) {
	if len(signer) != ed25519.PrivateKeySize || len(profile.Keys) == 0 || len(profile.Keys) > maximumClosedIssuerKeys {
		return nil, errors.New("closed issuer public profile is invalid")
	}
	raw := make([]byte, 0, 128+len(profile.Keys)*360)
	raw = append(raw, closedIssuerProfileMagic...)
	for _, item := range [][32]byte{profile.NetworkID, profile.NodeID} {
		raw = append(raw, item[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, uint64(profile.NotBefore.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(profile.NotAfter.Unix()))
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(profile.Keys)))
	for index, key := range profile.Keys {
		if key.WindowStart.Truncate(time.Hour) != key.WindowStart || key.WindowStart.Before(profile.NotBefore) || !key.WindowStart.Before(profile.NotAfter) ||
			key.Class < 1 || key.Class > 3 || len(key.SPKI) != 346 || !state.ValidateClosedTokenSPKI(key.SPKI) || key.KeyID != sha256.Sum256(key.SPKI) ||
			index > 0 && (profile.Keys[index-1].WindowStart.After(key.WindowStart) ||
				profile.Keys[index-1].WindowStart.Equal(key.WindowStart) && profile.Keys[index-1].Class >= key.Class) {
			return nil, errors.New("closed issuer public profile keys are invalid")
		}
		raw = binary.BigEndian.AppendUint64(raw, uint64(key.WindowStart.Unix()))
		raw = append(raw, key.Class)
		raw = binary.BigEndian.AppendUint16(raw, uint16(len(key.SPKI)))
		raw = append(raw, key.SPKI...)
	}
	return append(raw, ed25519.Sign(signer, closedIssuerTranscript(raw))...), nil
}

func closedIssuerTranscript(body []byte) []byte {
	transcript := make([]byte, 0, len(closedIssuerDomain)+len(body))
	transcript = append(transcript, closedIssuerDomain...)
	return append(transcript, body...)
}

// DecodeClosedIssuerProfile verifies one exact Node-signed public key profile.
func DecodeClosedIssuerProfile(raw []byte, nodePublic ed25519.PublicKey) (ClosedIssuerProfile, error) {
	if len(nodePublic) != ed25519.PublicKeySize || len(raw) < len(closedIssuerProfileMagic)+32+32+8+8+2+ed25519.SignatureSize ||
		!ed25519.Verify(nodePublic, closedIssuerTranscript(raw[:len(raw)-ed25519.SignatureSize]), raw[len(raw)-ed25519.SignatureSize:]) {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile signature is invalid")
	}
	body := raw[:len(raw)-ed25519.SignatureSize]
	if string(body[:8]) != closedIssuerProfileMagic {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile magic is invalid")
	}
	profile := ClosedIssuerProfile{}
	offset := 8
	for _, item := range []*[32]byte{&profile.NetworkID, &profile.NodeID} {
		copy(item[:], body[offset:offset+32])
		offset += 32
	}
	profile.NotBefore = time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()
	offset += 8
	profile.NotAfter = time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()
	offset += 8
	count := int(binary.BigEndian.Uint16(body[offset : offset+2]))
	offset += 2
	if profile.NetworkID == [32]byte{} || profile.NodeID == [32]byte{} || count == 0 || count > maximumClosedIssuerKeys || count != closedIssuerKeyCount(profile.NotBefore, profile.NotAfter) {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile facts are invalid")
	}
	for index := 0; index < count; index++ {
		if offset+11 > len(body) {
			return ClosedIssuerProfile{}, errors.New("closed issuer public profile is truncated")
		}
		key := ClosedIssuerKey{WindowStart: time.Unix(int64(binary.BigEndian.Uint64(body[offset:offset+8])), 0).UTC()}
		offset += 8
		key.Class = body[offset]
		offset++
		length := int(binary.BigEndian.Uint16(body[offset : offset+2]))
		offset += 2
		if length != 346 || offset+length > len(body) {
			return ClosedIssuerProfile{}, errors.New("closed issuer public profile key is invalid")
		}
		key.SPKI = append([]byte(nil), body[offset:offset+length]...)
		offset += length
		key.KeyID = sha256.Sum256(key.SPKI)
		if key.WindowStart.Truncate(time.Hour) != key.WindowStart || key.WindowStart.Before(profile.NotBefore) || !key.WindowStart.Before(profile.NotAfter) ||
			key.Class < 1 || key.Class > 3 || !state.ValidateClosedTokenSPKI(key.SPKI) ||
			len(profile.Keys) > 0 && (profile.Keys[len(profile.Keys)-1].WindowStart.After(key.WindowStart) ||
				profile.Keys[len(profile.Keys)-1].WindowStart.Equal(key.WindowStart) && profile.Keys[len(profile.Keys)-1].Class >= key.Class) {
			return ClosedIssuerProfile{}, errors.New("closed issuer public profile keys are not canonical")
		}
		profile.Keys = append(profile.Keys, key)
	}
	if offset != len(body) {
		return ClosedIssuerProfile{}, errors.New("closed issuer public profile framing is invalid")
	}
	copy(profile.Signature[:], raw[len(body):])
	return profile, nil
}

func encodeClosedIssuerMaterial(material closedIssuerMaterial) ([]byte, error) {
	keys, err := closedIssuerPublicKeys(material)
	if err != nil || len(keys) != len(material.keys) {
		return nil, errors.New("closed issuer key material is invalid")
	}
	raw := make([]byte, 0, 128+len(material.keys)*1400)
	raw = append(raw, closedIssuerMaterialMagic...)
	for _, item := range [][32]byte{material.network, material.node, material.signer} {
		raw = append(raw, item[:]...)
	}
	raw = binary.BigEndian.AppendUint64(raw, uint64(material.notBefore.Unix()))
	raw = binary.BigEndian.AppendUint64(raw, uint64(material.notAfter.Unix()))
	raw = binary.BigEndian.AppendUint16(raw, uint16(len(material.keys)))
	for _, item := range material.keys {
		if len(item.der) == 0 || len(item.der) > 4096 {
			return nil, errors.New("closed issuer private material is invalid")
		}
		raw = binary.BigEndian.AppendUint64(raw, uint64(item.window.Unix()))
		raw = append(raw, item.class)
		raw = binary.BigEndian.AppendUint16(raw, uint16(len(item.der)))
		raw = append(raw, item.der...)
	}
	if len(raw) > maximumClosedIssuerMaterial {
		return nil, errors.New("closed issuer private material exceeds bound")
	}
	return raw, nil
}

func decodeClosedIssuerMaterial(raw []byte) (closedIssuerMaterial, error) {
	if len(raw) < 8+96+8+8+2 || len(raw) > maximumClosedIssuerMaterial || string(raw[:8]) != closedIssuerMaterialMagic {
		return closedIssuerMaterial{}, errors.New("closed issuer private material framing is invalid")
	}
	material := closedIssuerMaterial{}
	offset := 8
	for _, item := range []*[32]byte{&material.network, &material.node, &material.signer} {
		copy(item[:], raw[offset:offset+32])
		offset += 32
	}
	material.notBefore = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	material.notAfter = time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()
	offset += 8
	count := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
	offset += 2
	if material.network == [32]byte{} || material.node == [32]byte{} || material.signer == [32]byte{} || count == 0 || count > maximumClosedIssuerKeys {
		return closedIssuerMaterial{}, errors.New("closed issuer private material facts are invalid")
	}
	for index := 0; index < count; index++ {
		if offset+11 > len(raw) {
			return closedIssuerMaterial{}, errors.New("closed issuer private material is truncated")
		}
		item := closedIssuerPrivateKey{window: time.Unix(int64(binary.BigEndian.Uint64(raw[offset:offset+8])), 0).UTC()}
		offset += 8
		item.class = raw[offset]
		offset++
		length := int(binary.BigEndian.Uint16(raw[offset : offset+2]))
		offset += 2
		if length == 0 || length > 4096 || offset+length > len(raw) {
			return closedIssuerMaterial{}, errors.New("closed issuer private material key is invalid")
		}
		item.der = append([]byte(nil), raw[offset:offset+length]...)
		offset += length
		material.keys = append(material.keys, item)
	}
	if offset != len(raw) {
		return closedIssuerMaterial{}, errors.New("closed issuer private material framing is invalid")
	}
	if _, err := closedIssuerPublicKeys(material); err != nil {
		return closedIssuerMaterial{}, err
	}
	return material, nil
}
