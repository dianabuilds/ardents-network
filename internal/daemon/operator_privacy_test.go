package daemon

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"
	"time"

	runtimeconfig "ardents/internal/config"
	"ardents/internal/identity"
	identityapi "ardents/internal/identity"
	identitycapability "ardents/internal/identity/capability"
	identitykeyring "ardents/internal/identity/keyring"
	identityprincipal "ardents/internal/identity/principal"
	apppolicy "ardents/internal/policy"

	"github.com/stretchr/testify/require"
)

func TestOperatorPrivacyChannelsLoadProtectedProvisionedChannels(t *testing.T) {
	doc := provisionOperatorPrivacy(t)
	cfg, err := runtimeConfigFromDocument(doc, "operator-token")
	require.NoError(t, err)
	require.NotNil(t, cfg.Node.Privacy)
	require.NotNil(t, cfg.Node.DataPrivacy)
	discoveryTopic, err := cfg.Node.Privacy.ContentTopic()
	require.NoError(t, err)
	dataTopic, err := cfg.Node.DataPrivacy.ContentTopic()
	require.NoError(t, err)
	require.NotEqual(t, discoveryTopic, dataTopic)
	require.NotContains(t, discoveryTopic, "discovery")
	require.NotContains(t, dataTopic, "data")
}

func TestOperatorPrivacyRejectsIdentitySubjectMismatch(t *testing.T) {
	doc := provisionOperatorPrivacy(t)
	doc.Privacy.Subject = "p_wrong"
	_, err := runtimeConfigFromDocument(doc, "operator-token")
	require.ErrorContains(t, err, "privacy.subject")
}

func TestOperatorPrivacyRejectsWrongStoreKeyWithoutLeakingMaterial(t *testing.T) {
	doc := provisionOperatorPrivacy(t)
	wrong := bytes.Repeat([]byte{0xee}, 32)
	require.NoError(t, os.WriteFile(doc.Privacy.CapabilityStoreKeyFile,
		[]byte(base64.StdEncoding.EncodeToString(wrong)), 0o600))
	_, err := runtimeConfigFromDocument(doc, "operator-token")
	require.EqualError(t, err, "protected privacy capability store is unavailable or invalid")
	require.NotContains(t, err.Error(), doc.Privacy.CapabilityStore)
	require.NotContains(t, err.Error(), base64.StdEncoding.EncodeToString(wrong))
}

func TestOperatorPrivacyRequiresPrivateKeyFilePermissions(t *testing.T) {
	doc := provisionOperatorPrivacy(t)
	require.NoError(t, os.Chmod(doc.Privacy.ReplayKeyFile, 0o644))
	_, err := runtimeConfigFromDocument(doc, "operator-token")
	require.ErrorContains(t, err, "permissions")
	require.NotContains(t, err.Error(), doc.Privacy.ReplayKeyFile)
}

func provisionOperatorPrivacy(t *testing.T) runtimeconfig.Document {
	t.Helper()
	dir := t.TempDir()
	state := identity.NewStoreInDir(dir)
	keys := identitykeyring.NewKeyStoreInDir(dir)
	summary, _, err := identityapi.NewService().EnsureNode(state, keys)
	require.NoError(t, err)
	subject := summary.Principal

	issuerPrivate := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x41}, ed25519.SeedSize))
	issuerPublic := issuerPrivate.Public().(ed25519.PublicKey)
	issuer := identityprincipal.DeriveID("p", issuerPublic)
	storeKey := bytes.Repeat([]byte{0x51}, 32)
	replayKey := bytes.Repeat([]byte{0x61}, 32)
	storePath := filepath.Join(dir, "capabilities.db")
	Workloads, err := identitycapability.NewService(
		storePath, storeKey, subject, map[string]ed25519.PublicKey{issuer: issuerPublic},
		apppolicy.New(apppolicy.Config{}), time.Now,
	)
	require.NoError(t, err)
	now := time.Now().UTC().Truncate(time.Second)
	discoveryRef := importOperatorGrant(t, Workloads, issuerPrivate, issuer, subject, 0x71, 0x81,
		identityapi.CapabilityRealmDiscovery, now)
	dataRef := importOperatorGrant(t, Workloads, issuerPrivate, issuer, subject, 0x72, 0x82,
		identityapi.CapabilityDataExchange, now)
	storeKeyPath := writeOperatorKey(t, dir, "capability-store.key", storeKey)
	replayKeyPath := writeOperatorKey(t, dir, "replay.key", replayKey)

	doc := runtimeconfig.Defaults()
	doc.Node.DataDir = dir
	doc.Privacy = runtimeconfig.PrivacyConfig{
		Required: true, CapabilityStore: storePath, CapabilityStoreKeyFile: storeKeyPath,
		ReplayKeyFile: replayKeyPath, Subject: subject,
		TrustedIssuers: map[string]string{issuer: base64.StdEncoding.EncodeToString(issuerPublic)},
		Discovery: runtimeconfig.PrivacyChannelConfig{
			Reference: string(discoveryRef), ReplayPath: filepath.Join(dir, "discovery-replay.db"),
		},
		Data: runtimeconfig.PrivacyChannelConfig{
			Reference: string(dataRef), ReplayPath: filepath.Join(dir, "data-replay.db"),
		},
	}
	return doc
}

func importOperatorGrant(
	t *testing.T,
	Workloads *identitycapability.Service,
	issuerPrivate ed25519.PrivateKey,
	issuer, subject string,
	channelByte, grantByte byte,
	scope identityapi.CapabilityScope,
	now time.Time,
) identityapi.CapabilityRef {
	t.Helper()
	secret, ok := identityapi.NewCapabilitySecret(bytes.Repeat([]byte{channelByte}, 32))
	require.True(t, ok)
	grant, err := identitycapability.SignGrant(identityapi.CapabilityGrant{
		Version: 1, ChannelID: operatorPrivacyID(channelByte), Generation: 1, Secret: secret,
		GrantID: operatorPrivacyID(grantByte), IssuerPrincipal: issuer, SubjectPrincipal: subject,
		Permissions: identityapi.CapabilityPublish | identityapi.CapabilitySubscribe | identityapi.CapabilityStoreFetch,
		Scope:       scope, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}, issuerPrivate)
	require.NoError(t, err)
	ref, err := Workloads.ImportGrant(grant)
	require.NoError(t, err)
	return ref
}

func writeOperatorKey(t *testing.T, dir, name string, key []byte) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(key)), 0o600))
	return path
}

func operatorPrivacyID(value byte) [16]byte {
	var id [16]byte
	for index := range id {
		id[index] = value
	}
	return id
}
