// Package identity owns ardentsctl Principal and device key custody, typed
// signer adapters, Principal enrollment and access-administration commands, and
// public identity presentation. It does not own sessions or server-side state.
package identity

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	bundleVersion        = 1
	bundleAlgorithm      = "ed25519"
	rootBundleKind       = "principal-root"
	deviceBundleKind     = "device"
	maxSignerBundleSize  = 32 << 10
	defaultCredentialTTL = 90 * 24 * time.Hour
)

var (
	ErrSignerFileInvalid = errors.New("signer file is invalid")
	ErrSignerFileMissing = errors.New("signer file does not exist")
	ErrSignerFileExists  = errors.New("signer file already exists")
	ErrSignerFileUnsafe  = errors.New("signer file permissions are not private")
	ErrSignerUnavailable = errors.New("signer file is unavailable")
)

var (
	rootBundleFields   = []string{"version", "kind", "algorithm", "principal", "root_public_key", "root_private_seed"}
	deviceBundleFields = []string{"version", "kind", "algorithm", "principal", "root_public_key", "device_id", "device_public_key", "device_private_seed", "credential"}
)

type rootBundle struct {
	Version         uint32 `json:"version"`
	Kind            string `json:"kind"`
	Algorithm       string `json:"algorithm"`
	Principal       string `json:"principal"`
	RootPublicKey   string `json:"root_public_key"`
	RootPrivateSeed string `json:"root_private_seed"`
}

func (rootBundle) String() string   { return "Principal root signer bundle [redacted]" }
func (rootBundle) GoString() string { return "Principal root signer bundle [redacted]" }

type deviceBundle struct {
	Version           uint32 `json:"version"`
	Kind              string `json:"kind"`
	Algorithm         string `json:"algorithm"`
	Principal         string `json:"principal"`
	RootPublicKey     string `json:"root_public_key"`
	DeviceID          string `json:"device_id"`
	DevicePublicKey   string `json:"device_public_key"`
	DevicePrivateSeed string `json:"device_private_seed"`
	Credential        string `json:"credential"`
}

func (deviceBundle) String() string   { return "device signer bundle [redacted]" }
func (deviceBundle) GoString() string { return "device signer bundle [redacted]" }

type rootMaterial struct {
	principal string
	public    ed25519.PublicKey
	private   ed25519.PrivateKey
}

func (rootMaterial) String() string   { return "Principal root signer [redacted]" }
func (rootMaterial) GoString() string { return "Principal root signer [redacted]" }

type deviceMaterial struct {
	principal  string
	rootPublic ed25519.PublicKey
	deviceID   string
	public     ed25519.PublicKey
	private    ed25519.PrivateKey
	credential *identityaccess.Artifact
}

func (deviceMaterial) String() string   { return "device signer [redacted]" }
func (deviceMaterial) GoString() string { return "device signer [redacted]" }

type PrincipalInfo struct {
	Version       uint32 `json:"version"`
	Kind          string `json:"kind"`
	Algorithm     string `json:"algorithm"`
	Principal     string `json:"principal"`
	RootPublicKey string `json:"root_public_key"`
}

type DeviceInfo struct {
	Version             uint32    `json:"version"`
	Kind                string    `json:"kind"`
	Algorithm           string    `json:"algorithm"`
	Principal           string    `json:"principal"`
	RootPublicKey       string    `json:"root_public_key"`
	DeviceID            string    `json:"device_id"`
	DevicePublicKey     string    `json:"device_public_key"`
	CredentialID        string    `json:"credential_id"`
	CredentialNotBefore time.Time `json:"credential_not_before"`
	CredentialNotAfter  time.Time `json:"credential_not_after"`
}

func DefaultPrincipalSignerPath() (string, error) {
	return defaultSignerPath("principal-root-v1.json")
}

func DefaultDeviceSignerPath() (string, error) {
	return defaultSignerPath("device-v1.json")
}

func defaultSignerPath(name string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve signer directory: %w", err)
	}
	return filepath.Join(dir, "ardents", "identity", name), nil
}

func CreatePrincipal(path string, entropy io.Reader) (PrincipalInfo, error) {
	if path == "" || entropy == nil {
		return PrincipalInfo{}, ErrSignerFileInvalid
	}
	public, private, err := ed25519.GenerateKey(entropy)
	if err != nil {
		return PrincipalInfo{}, ErrSignerUnavailable
	}
	principalID, err := identityprincipal.FromEd25519PublicKey(public)
	if err != nil {
		return PrincipalInfo{}, ErrSignerFileInvalid
	}
	principal := principalID.String()
	bundle := rootBundle{
		Version: bundleVersion, Kind: rootBundleKind, Algorithm: bundleAlgorithm,
		Principal: principal, RootPublicKey: encode(public), RootPrivateSeed: encode(private.Seed()),
	}
	if err := createBundle(path, bundle); err != nil {
		return PrincipalInfo{}, err
	}
	return principalInfo(rootMaterial{principal: principal, public: public, private: private}), nil
}

func ImportPrincipal(source, destination string) (PrincipalInfo, error) {
	material, err := loadRootMaterial(source)
	if err != nil {
		return PrincipalInfo{}, err
	}
	bundle := rootBundle{
		Version: bundleVersion, Kind: rootBundleKind, Algorithm: bundleAlgorithm,
		Principal: material.principal, RootPublicKey: encode(material.public), RootPrivateSeed: encode(material.private.Seed()),
	}
	if err := createBundle(destination, bundle); err != nil {
		return PrincipalInfo{}, err
	}
	return principalInfo(material), nil
}

func ShowPrincipal(path string) (PrincipalInfo, error) {
	material, err := loadRootMaterial(path)
	if err != nil {
		return PrincipalInfo{}, err
	}
	return principalInfo(material), nil
}

func CreateDevice(rootPath, devicePath string, validity time.Duration, now time.Time, entropy io.Reader) (DeviceInfo, error) {
	if devicePath == "" || entropy == nil || validity <= 0 || validity > identitycontract.MaxCredentialLifetime {
		return DeviceInfo{}, ErrSignerFileInvalid
	}
	root, err := loadRootMaterial(rootPath)
	if err != nil {
		return DeviceInfo{}, err
	}
	public, private, err := ed25519.GenerateKey(entropy)
	if err != nil {
		return DeviceInfo{}, ErrSignerUnavailable
	}
	derivedDevice, err := identityprincipal.DeviceFromEd25519PublicKey(public)
	if err != nil {
		return DeviceInfo{}, ErrSignerFileInvalid
	}
	deviceID := derivedDevice.String()
	notBefore := now.UTC().Truncate(time.Second)
	notAfter := notBefore.Add(validity).Truncate(time.Second)
	if !notAfter.After(notBefore) {
		return DeviceInfo{}, ErrSignerFileInvalid
	}
	credential, err := identityaccess.SignKeyCredential(&identityprotocol.KeyCredentialPayload{
		Version: identitycontract.Version, Subject: root.principal, RootPublicKey: append([]byte(nil), root.public...),
		DeviceId: deviceID, DevicePublicKey: append([]byte(nil), public...),
		Purposes:  []identityprotocol.CredentialPurpose{identityprotocol.CredentialPurpose_CREDENTIAL_PURPOSE_AUTHENTICATE},
		NotBefore: timestamppb.New(notBefore), NotAfter: timestamppb.New(notAfter),
	}, root.private)
	if err != nil {
		return DeviceInfo{}, ErrSignerFileInvalid
	}
	credentialRaw, err := credential.MarshalBinary()
	if err != nil {
		return DeviceInfo{}, ErrSignerFileInvalid
	}
	bundle := deviceBundle{
		Version: bundleVersion, Kind: deviceBundleKind, Algorithm: bundleAlgorithm,
		Principal: root.principal, RootPublicKey: encode(root.public), DeviceID: deviceID,
		DevicePublicKey: encode(public), DevicePrivateSeed: encode(private.Seed()), Credential: encode(credentialRaw),
	}
	if err := createBundle(devicePath, bundle); err != nil {
		return DeviceInfo{}, err
	}
	return deviceInfo(deviceMaterial{principal: root.principal, rootPublic: root.public, deviceID: deviceID, public: public, private: private, credential: credential}), nil
}

func ShowDevice(path string) (DeviceInfo, error) {
	material, err := loadDeviceMaterial(path)
	if err != nil {
		return DeviceInfo{}, err
	}
	return deviceInfo(material), nil
}

func loadRootMaterial(path string) (rootMaterial, error) {
	raw, err := readBundle(path)
	if err != nil {
		return rootMaterial{}, err
	}
	var bundle rootBundle
	if strictJSON(raw, &bundle, rootBundleFields) != nil || bundle.Version != bundleVersion || bundle.Kind != rootBundleKind || bundle.Algorithm != bundleAlgorithm {
		return rootMaterial{}, ErrSignerFileInvalid
	}
	public, err := decodeExact(bundle.RootPublicKey, ed25519.PublicKeySize)
	if err != nil {
		return rootMaterial{}, ErrSignerFileInvalid
	}
	seed, err := decodeExact(bundle.RootPrivateSeed, ed25519.SeedSize)
	if err != nil {
		return rootMaterial{}, ErrSignerFileInvalid
	}
	private := ed25519.NewKeyFromSeed(seed)
	if !bytes.Equal(private.Public().(ed25519.PublicKey), public) {
		return rootMaterial{}, ErrSignerFileInvalid
	}
	derivedPrincipal, err := identityprincipal.FromEd25519PublicKey(public)
	if err != nil || derivedPrincipal.String() != bundle.Principal {
		return rootMaterial{}, ErrSignerFileInvalid
	}
	principal := derivedPrincipal.String()
	return rootMaterial{principal: principal, public: public, private: private}, nil
}

func loadDeviceMaterial(path string) (deviceMaterial, error) {
	raw, err := readBundle(path)
	if err != nil {
		return deviceMaterial{}, err
	}
	var bundle deviceBundle
	if strictJSON(raw, &bundle, deviceBundleFields) != nil || bundle.Version != bundleVersion || bundle.Kind != deviceBundleKind || bundle.Algorithm != bundleAlgorithm {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	rootPublic, err := decodeExact(bundle.RootPublicKey, ed25519.PublicKeySize)
	if err != nil {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	derivedPrincipal, err := identityprincipal.FromEd25519PublicKey(rootPublic)
	if err != nil || derivedPrincipal.String() != bundle.Principal {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	principal := derivedPrincipal.String()
	public, err := decodeExact(bundle.DevicePublicKey, ed25519.PublicKeySize)
	if err != nil {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	seed, err := decodeExact(bundle.DevicePrivateSeed, ed25519.SeedSize)
	if err != nil {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	private := ed25519.NewKeyFromSeed(seed)
	if !bytes.Equal(private.Public().(ed25519.PublicKey), public) {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	derivedDevice, err := identityprincipal.DeviceFromEd25519PublicKey(public)
	if err != nil || derivedDevice.String() != bundle.DeviceID {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	deviceID := derivedDevice.String()
	credentialRaw, err := decodeExactRange(bundle.Credential, 1, identitycontract.MaxKeyCredentialBytes)
	if err != nil {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	credential, err := identityaccess.ParseAndVerifyKeyCredential(credentialRaw, time.Time{})
	if err != nil {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	view := credential.KeyCredentialPayload()
	if view == nil || view.GetSubject() != principal || view.GetDeviceId() != deviceID || !bytes.Equal(view.GetRootPublicKey(), rootPublic) || !bytes.Equal(view.GetDevicePublicKey(), public) {
		return deviceMaterial{}, ErrSignerFileInvalid
	}
	return deviceMaterial{principal: principal, rootPublic: rootPublic, deviceID: deviceID, public: public, private: private, credential: credential}, nil
}

func principalInfo(material rootMaterial) PrincipalInfo {
	return PrincipalInfo{Version: bundleVersion, Kind: "principal", Algorithm: bundleAlgorithm, Principal: material.principal, RootPublicKey: encode(material.public)}
}

func deviceInfo(material deviceMaterial) DeviceInfo {
	credential := material.credential.KeyCredentialPayload()
	return DeviceInfo{
		Version: bundleVersion, Kind: deviceBundleKind, Algorithm: bundleAlgorithm,
		Principal: material.principal, RootPublicKey: encode(material.rootPublic), DeviceID: material.deviceID,
		DevicePublicKey: encode(material.public), CredentialID: material.credential.ID(),
		CredentialNotBefore: credential.GetNotBefore().AsTime().UTC(), CredentialNotAfter: credential.GetNotAfter().AsTime().UTC(),
	}
}

func createBundle(path string, bundle any) error {
	raw, err := json.Marshal(bundle)
	if err != nil {
		return ErrSignerFileInvalid
	}
	raw = append(raw, '\n')
	if len(raw) > maxSignerBundleSize {
		return ErrSignerFileInvalid
	}
	if err := storage.AtomicCreatePrivateFile(path, raw); err != nil {
		if errors.Is(err, os.ErrExist) {
			return ErrSignerFileExists
		}
		return ErrSignerUnavailable
	}
	return nil
}

func readBundle(path string) ([]byte, error) {
	if path == "" {
		return nil, ErrSignerFileInvalid
	}
	if err := storage.ValidatePrivateDir(filepath.Dir(path)); err != nil {
		if unsafeStorageProtection(err) {
			return nil, ErrSignerFileUnsafe
		}
		return nil, ErrSignerUnavailable
	}
	raw, found, err := storage.ReadStrictPrivateFileBounded(path, maxSignerBundleSize)
	if err != nil {
		if unsafeStorageProtection(err) {
			return nil, ErrSignerFileUnsafe
		}
		if strings.Contains(err.Error(), "size limit") {
			return nil, fmt.Errorf("%w: size limit exceeded", ErrSignerFileInvalid)
		}
		return nil, ErrSignerUnavailable
	}
	if !found {
		return nil, ErrSignerFileMissing
	}
	return raw, nil
}

func unsafeStorageProtection(err error) bool {
	message := err.Error()
	return strings.Contains(message, "permissions allow") || strings.Contains(message, "ACL")
}

func strictJSON(raw []byte, target any, allowedFields []string) error {
	if len(raw) == 0 || rejectDuplicateFields(raw) != nil {
		return ErrSignerFileInvalid
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || len(object) != len(allowedFields) {
		return ErrSignerFileInvalid
	}
	allowed := make(map[string]struct{}, len(allowedFields))
	for _, field := range allowedFields {
		allowed[field] = struct{}{}
	}
	for field := range object {
		if _, ok := allowed[field]; !ok {
			return ErrSignerFileInvalid
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return ErrSignerFileInvalid
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return ErrSignerFileInvalid
	}
	return nil
}

func rejectDuplicateFields(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := consumeJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return ErrSignerFileInvalid
	}
	return nil
}

func consumeJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]struct{})
		for decoder.More() {
			nameToken, err := decoder.Token()
			if err != nil {
				return err
			}
			name, ok := nameToken.(string)
			if !ok {
				return ErrSignerFileInvalid
			}
			if _, exists := seen[name]; exists {
				return ErrSignerFileInvalid
			}
			seen[name] = struct{}{}
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim('}') {
			return ErrSignerFileInvalid
		}
	case '[':
		for decoder.More() {
			if err := consumeJSONValue(decoder); err != nil {
				return err
			}
		}
		closing, err := decoder.Token()
		if err != nil || closing != json.Delim(']') {
			return ErrSignerFileInvalid
		}
	default:
		return ErrSignerFileInvalid
	}
	return nil
}

func encode(raw []byte) string { return base64.RawURLEncoding.EncodeToString(raw) }

func decodeExact(value string, size int) ([]byte, error) {
	return decodeExactRange(value, size, size)
}

func decodeExactRange(value string, minimum, maximum int) ([]byte, error) {
	if value == "" {
		return nil, ErrSignerFileInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(raw) < minimum || len(raw) > maximum || encode(raw) != value {
		return nil, ErrSignerFileInvalid
	}
	return raw, nil
}
