package identity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	identityaccess "ardents/internal/identity/access"
	identityprincipal "ardents/internal/identity/principal"
	identityprotocol "ardents/internal/identity/protocol"
	"ardents/internal/storage"

	"github.com/stretchr/testify/require"
)

var signerTestNow = time.Date(2031, 2, 3, 4, 5, 6, 0, time.UTC)

func TestPrincipalAndDeviceBundlesRoundTripAndTypedSigners(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")

	principalInfo, err := CreatePrincipal(rootPath, bytes.NewReader(bytes.Repeat([]byte{0x11}, ed25519.SeedSize)))
	require.NoError(t, err)
	deviceInfo, err := CreateDevice(rootPath, devicePath, defaultCredentialTTL, signerTestNow, bytes.NewReader(bytes.Repeat([]byte{0x22}, ed25519.SeedSize)))
	require.NoError(t, err)
	require.Equal(t, principalInfo.Principal, deviceInfo.Principal)
	require.True(t, strings.HasPrefix(principalInfo.Principal, "p1_"))
	require.True(t, strings.HasPrefix(deviceInfo.DeviceID, "d1_"))
	require.True(t, strings.HasPrefix(deviceInfo.CredentialID, "kc1_"))
	require.Equal(t, signerTestNow, deviceInfo.CredentialNotBefore)
	require.Equal(t, signerTestNow.Add(defaultCredentialTTL), deviceInfo.CredentialNotAfter)

	// Reloading simulates restart and returns exactly the same public identity.
	reloadedPrincipal, err := ShowPrincipal(rootPath)
	require.NoError(t, err)
	reloadedDevice, err := ShowDevice(devicePath)
	require.NoError(t, err)
	require.Equal(t, principalInfo, reloadedPrincipal)
	require.Equal(t, deviceInfo, reloadedDevice)

	rootSigner, err := OpenRootFileSigner(rootPath)
	require.NoError(t, err)
	deviceSigner, err := OpenDeviceFileSigner(devicePath)
	require.NoError(t, err)
	credential, err := deviceSigner.Credential(context.Background())
	require.NoError(t, err)
	view := credential.KeyCredentialPayload()
	require.Equal(t, principalInfo.Principal, view.GetSubject())
	require.Equal(t, deviceInfo.DeviceID, view.GetDeviceId())

	issued, err := rootSigner.IssueKeyCredential(context.Background(), KeyCredentialSpec{
		Subject: principalInfo.Principal, RootPublicKey: view.GetRootPublicKey(), DeviceID: deviceInfo.DeviceID,
		DevicePublicKey: view.GetDevicePublicKey(), NotBefore: signerTestNow, NotAfter: signerTestNow.Add(defaultCredentialTTL),
	})
	require.NoError(t, err)
	require.Equal(t, credential.ID(), issued.ID())

	sessionChallenge := testChallenge(t, principalInfo.Principal, identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)
	signature, err := deviceSigner.SignAuthenticationChallenge(context.Background(), sessionChallenge)
	require.NoError(t, err)
	require.Len(t, signature, ed25519.SignatureSize)
	require.Equal(t, signature, mustSignAuthentication(t, deviceSigner, sessionChallenge))

	enrollmentChallenge := testChallenge(t, principalInfo.Principal, identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF)
	enrollmentSignature, err := rootSigner.SignEnrollmentChallenge(context.Background(), enrollmentChallenge)
	require.NoError(t, err)
	require.Len(t, enrollmentSignature, ed25519.SignatureSize)

	_, err = deviceSigner.SignAuthenticationChallenge(context.Background(), enrollmentChallenge)
	require.Error(t, err)
	_, err = rootSigner.SignEnrollmentChallenge(context.Background(), sessionChallenge)
	require.Error(t, err)

	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = deviceSigner.Principal(cancelled)
	require.ErrorIs(t, err, context.Canceled)

	assertPrivateFile(t, rootPath)
	assertPrivateFile(t, devicePath)
}

func TestDeterministicMemorySignerMatchesProtectedFileSigner(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")
	_, err := CreatePrincipal(rootPath, bytes.NewReader(bytes.Repeat([]byte{0x31}, ed25519.SeedSize)))
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, time.Hour, signerTestNow, bytes.NewReader(bytes.Repeat([]byte{0x32}, ed25519.SeedSize)))
	require.NoError(t, err)

	root := mustLoadRoot(t, rootPath)
	device := mustLoadDevice(t, devicePath)
	memory := memorySigner{root: root, device: device}
	var _ SessionSigner = memory
	var _ EnrollmentSigner = memory
	fileDevice, err := OpenDeviceFileSigner(devicePath)
	require.NoError(t, err)
	fileRoot, err := OpenRootFileSigner(rootPath)
	require.NoError(t, err)

	session := testChallenge(t, root.principal, identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_SESSION)
	want, err := fileDevice.SignAuthenticationChallenge(context.Background(), session)
	require.NoError(t, err)
	got, err := memory.SignAuthenticationChallenge(context.Background(), session)
	require.NoError(t, err)
	require.Equal(t, want, got)

	enrollment := testChallenge(t, root.principal, identityprotocol.ChallengePurpose_CHALLENGE_PURPOSE_ENROLLMENT_PROOF)
	want, err = fileRoot.SignEnrollmentChallenge(context.Background(), enrollment)
	require.NoError(t, err)
	got, err = memory.SignEnrollmentChallenge(context.Background(), enrollment)
	require.NoError(t, err)
	require.Equal(t, want, got)
}

func TestPrincipalImportIsValidatedAtomicAndNoReplace(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "identity")
	source := filepath.Join(dir, "source.json")
	destination := filepath.Join(dir, "imported.json")
	info, err := CreatePrincipal(source, bytes.NewReader(bytes.Repeat([]byte{0x41}, ed25519.SeedSize)))
	require.NoError(t, err)

	imported, err := ImportPrincipal(source, destination)
	require.NoError(t, err)
	require.Equal(t, info, imported)
	before, found, err := storage.ReadPrivateFile(destination)
	require.NoError(t, err)
	require.True(t, found)

	_, err = ImportPrincipal(source, destination)
	require.ErrorIs(t, err, ErrSignerFileExists)
	after, found, err := storage.ReadPrivateFile(destination)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, before, after)
}

func TestSignerBundlesRejectMalformedUnknownDuplicateAndMismatchedState(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "identity")
	source := filepath.Join(dir, "source.json")
	_, err := CreatePrincipal(source, bytes.NewReader(bytes.Repeat([]byte{0x51}, ed25519.SeedSize)))
	require.NoError(t, err)
	raw, found, err := storage.ReadPrivateFile(source)
	require.NoError(t, err)
	require.True(t, found)

	var valid map[string]any
	require.NoError(t, json.Unmarshal(raw, &valid))
	mutated := func(change func(map[string]any)) []byte {
		clone := make(map[string]any, len(valid))
		for key, value := range valid {
			clone[key] = value
		}
		change(clone)
		result, marshalErr := json.Marshal(clone)
		require.NoError(t, marshalErr)
		return result
	}

	tests := map[string][]byte{
		"empty":           nil,
		"truncated":       raw[:len(raw)/2],
		"trailing":        append(append([]byte(nil), raw...), []byte("{}")...),
		"duplicate":       bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":1,"version":1`), 1),
		"unknown":         mutated(func(value map[string]any) { value["future"] = true }),
		"case_alias":      bytes.Replace(raw, []byte(`"version":1`), []byte(`"VERSION":1`), 1),
		"case_duplicate":  bytes.Replace(raw, []byte(`"version":1`), []byte(`"version":2,"VERSION":1`), 1),
		"wrong_version":   mutated(func(value map[string]any) { value["version"] = 2 }),
		"wrong_algorithm": mutated(func(value map[string]any) { value["algorithm"] = "future" }),
		"padded_seed":     mutated(func(value map[string]any) { value["root_private_seed"] = value["root_private_seed"].(string) + "=" }),
		"wrong_principal": mutated(func(value map[string]any) { value["principal"] = strings.Repeat("p", 55) }),
		"public_mismatch": mutated(func(value map[string]any) {
			value["root_public_key"] = encode(bytes.Repeat([]byte{0x61}, ed25519.PublicKeySize))
		}),
	}
	for name, malformed := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name+".json")
			require.NoError(t, storage.AtomicCreatePrivateFile(path, malformed))
			_, err := ShowPrincipal(path)
			require.ErrorIs(t, err, ErrSignerFileInvalid)
			if len(malformed) > 0 {
				require.NotContains(t, err.Error(), string(malformed))
			}
		})
	}

	oversize := filepath.Join(dir, "oversize.json")
	require.NoError(t, storage.AtomicCreatePrivateFile(oversize, bytes.Repeat([]byte{'x'}, maxSignerBundleSize+1)))
	_, err = ShowPrincipal(oversize)
	require.ErrorIs(t, err, ErrSignerFileInvalid)
	require.ErrorContains(t, err, "size limit")
	require.NotContains(t, err.Error(), strings.Repeat("x", 32))
}

func TestDeviceBundleRejectsCredentialAndKeySubstitution(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")
	_, err := CreatePrincipal(rootPath, bytes.NewReader(bytes.Repeat([]byte{0x71}, ed25519.SeedSize)))
	require.NoError(t, err)
	_, err = CreateDevice(rootPath, devicePath, time.Hour, signerTestNow, bytes.NewReader(bytes.Repeat([]byte{0x72}, ed25519.SeedSize)))
	require.NoError(t, err)
	raw, found, err := storage.ReadPrivateFile(devicePath)
	require.NoError(t, err)
	require.True(t, found)
	var bundle map[string]any
	require.NoError(t, json.Unmarshal(raw, &bundle))

	for name, change := range map[string]func(map[string]any){
		"device_id": func(value map[string]any) { value["device_id"] = strings.Repeat("d", 55) },
		"device_public": func(value map[string]any) {
			value["device_public_key"] = encode(bytes.Repeat([]byte{0x73}, ed25519.PublicKeySize))
		},
		"credential": func(value map[string]any) { value["credential"] = encode([]byte{0xff, 0x00}) },
	} {
		t.Run(name, func(t *testing.T) {
			copyMap := make(map[string]any, len(bundle))
			for key, value := range bundle {
				copyMap[key] = value
			}
			change(copyMap)
			malformed, marshalErr := json.Marshal(copyMap)
			require.NoError(t, marshalErr)
			path := filepath.Join(dir, "bad-"+name+".json")
			require.NoError(t, storage.AtomicCreatePrivateFile(path, malformed))
			_, err := ShowDevice(path)
			require.ErrorIs(t, err, ErrSignerFileInvalid)
		})
	}
}

func TestPublicMetadataNeverContainsPrivateSeedsOrCredentialBytes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "identity")
	rootPath := filepath.Join(dir, "root.json")
	devicePath := filepath.Join(dir, "device.json")
	principal, err := CreatePrincipal(rootPath, bytes.NewReader(bytes.Repeat([]byte{0x81}, ed25519.SeedSize)))
	require.NoError(t, err)
	device, err := CreateDevice(rootPath, devicePath, time.Hour, signerTestNow, bytes.NewReader(bytes.Repeat([]byte{0x82}, ed25519.SeedSize)))
	require.NoError(t, err)
	rootRaw, _, err := storage.ReadPrivateFile(rootPath)
	require.NoError(t, err)
	deviceRaw, _, err := storage.ReadPrivateFile(devicePath)
	require.NoError(t, err)
	var rootBundle rootBundle
	var deviceBundle deviceBundle
	require.NoError(t, json.Unmarshal(rootRaw, &rootBundle))
	require.NoError(t, json.Unmarshal(deviceRaw, &deviceBundle))

	public, err := json.Marshal(struct {
		Principal PrincipalInfo `json:"principal"`
		Device    DeviceInfo    `json:"device"`
	}{principal, device})
	require.NoError(t, err)
	for _, secret := range []string{rootBundle.RootPrivateSeed, deviceBundle.DevicePrivateSeed, deviceBundle.Credential} {
		require.NotEmpty(t, secret)
		require.NotContains(t, string(public), secret)
	}
	rootSigner, err := OpenRootFileSigner(rootPath)
	require.NoError(t, err)
	deviceSigner, err := OpenDeviceFileSigner(devicePath)
	require.NoError(t, err)
	formatted := fmt.Sprintf("%v %#v %v %#v", rootSigner, rootSigner, deviceSigner, deviceSigner)
	for _, secret := range []string{rootBundle.RootPrivateSeed, deviceBundle.DevicePrivateSeed, deviceBundle.Credential} {
		require.NotContains(t, formatted, secret)
	}
	require.Contains(t, formatted, "[redacted]")
}

type memorySigner struct {
	root   rootMaterial
	device deviceMaterial
}

func (s memorySigner) Principal(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return s.root.principal, nil
}

func (s memorySigner) Credential(ctx context.Context) (*identityaccess.Artifact, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return s.device.credential, nil
}

func (s memorySigner) SignAuthenticationChallenge(ctx context.Context, challenge identityaccess.Challenge) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return identityaccess.SignAuthenticationChallenge(challenge, s.device.credential, s.device.private)
}

func (s memorySigner) IssueKeyCredential(ctx context.Context, spec KeyCredentialSpec) (*identityaccess.Artifact, error) {
	return (&RootFileSigner{material: s.root}).IssueKeyCredential(ctx, spec)
}

func (s memorySigner) SignEnrollmentChallenge(ctx context.Context, challenge identityaccess.Challenge) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return identityaccess.SignEnrollmentChallenge(challenge, s.root.private)
}

func testChallenge(t *testing.T, principal string, purpose identityprotocol.ChallengePurpose) identityaccess.Challenge {
	t.Helper()
	nodeKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x91}, ed25519.SeedSize))
	node, err := identityprincipal.FromEd25519PublicKey(nodeKey.Public().(ed25519.PublicKey))
	require.NoError(t, err)
	var id identityaccess.ChallengeID
	var nonce, peer [32]byte
	copy(id[:], bytes.Repeat([]byte{0x92}, len(id)))
	copy(nonce[:], bytes.Repeat([]byte{0x93}, len(nonce)))
	copy(peer[:], bytes.Repeat([]byte{0x94}, len(peer)))
	return identityaccess.Challenge{
		Version: identitycontract.Version, ID: id, Nonce: nonce, Principal: principal,
		Binding: identityaccess.AuthenticationBinding{
			Audience:         identityaccess.Audience{Node: node.String(), Interface: identityprotocol.Interface_INTERFACE_OPERATOR, ProtocolMajor: identitycontract.ProtocolMajor},
			TransportProfile: identityprotocol.TransportProfile_TRANSPORT_PROFILE_UNIX_LOCAL_V1, PeerBinding: peer,
		},
		Purpose: purpose, IssuedAt: signerTestNow, ExpiresAt: signerTestNow.Add(identitycontract.ChallengeLifetime),
	}
}

func mustLoadRoot(t *testing.T, path string) rootMaterial {
	t.Helper()
	material, err := loadRootMaterial(path)
	require.NoError(t, err)
	return material
}

func mustLoadDevice(t *testing.T, path string) deviceMaterial {
	t.Helper()
	material, err := loadDeviceMaterial(path)
	require.NoError(t, err)
	return material
}

func mustSignAuthentication(t *testing.T, signer SessionSigner, challenge identityaccess.Challenge) []byte {
	t.Helper()
	signature, err := signer.SignAuthenticationChallenge(context.Background(), challenge)
	require.NoError(t, err)
	return signature
}

func assertPrivateFile(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	if os.PathSeparator != '\\' {
		require.Zero(t, info.Mode().Perm()&0o077)
	}
}

func TestMissingAndNonRegularSignerFilesFailClosed(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "identity")
	require.NoError(t, storage.EnsurePrivateDir(dir))
	_, err := ShowPrincipal(filepath.Join(dir, "missing.json"))
	require.ErrorIs(t, err, ErrSignerFileMissing)
	_, err = ShowPrincipal(t.TempDir())
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrSignerFileMissing))
}
