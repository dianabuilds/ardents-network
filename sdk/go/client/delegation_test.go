package client

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	identitycontract "ardents/api/ardents/identity/v1"
	sdkidentity "ardents/sdk/go/identity"

	"github.com/stretchr/testify/require"
)

type delegationSignerFunc func(context.Context, DelegationProposal) (*sdkidentity.Artifact, error)

func (f delegationSignerFunc) SignDelegation(ctx context.Context, proposal DelegationProposal) (*sdkidentity.Artifact, error) {
	return f(ctx, proposal)
}

func TestDelegationProposalDisplaysExactCanonicalConsentAndCreatesOpaqueArtifact(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	delegator, credential, device := delegationIdentity(t, now, 21, 22)
	application, _, _ := delegationIdentity(t, now, 23, 24)
	node, _, _ := delegationIdentity(t, now, 25, 26)

	proposal, err := NewDelegationProposal(DelegationRequest{
		DelegatorPrincipal:   delegator,
		ApplicationPrincipal: application,
		NodePrincipal:        node,
		Actions:              []string{"application.content.put", "application.content.get"},
		Scope:                sdkidentity.ResourceScope{Kind: sdkidentity.ScopePrincipalOwned, Owner: mustClientResourceOwner(t, delegator)},
		NotBefore:            now,
		NotAfter:             now.Add(15 * time.Minute),
	})
	require.NoError(t, err)
	require.Equal(t, "Ardents Delegation Consent v1\n"+
		"Delegator Principal: "+delegator+"\n"+
		"Node Principal: "+node+"\n"+
		"Application Principal: "+application+"\n"+
		"Actions:\n"+
		"- application.content.get\n"+
		"- application.content.put\n"+
		"Resource scope: principal-owned(owner="+delegator+")\n"+
		"Valid from: "+now.Format(time.RFC3339)+"\n"+
		"Expires at: "+now.Add(15*time.Minute).Format(time.RFC3339)+"\n"+
		"Redelegation: forbidden (one hop only)", proposal.ConsentText())

	artifact, err := CreateDelegation(context.Background(), proposal, delegationSignerFunc(func(_ context.Context, received DelegationProposal) (*sdkidentity.Artifact, error) {
		return sdkidentity.SignDelegation(received.Spec(credential), device, now)
	}))
	require.NoError(t, err)
	require.Equal(t, sdkidentity.KindDelegation, artifact.Kind())
	require.Contains(t, artifact.String(), "[redacted]")
	require.NotContains(t, artifact.String(), artifact.Delegation().CredentialID)
}

func TestDelegationProposalRejectsNoncanonicalOrOverbroadConsent(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	delegator, _, _ := delegationIdentity(t, now, 31, 32)
	application, _, _ := delegationIdentity(t, now, 33, 34)
	node, _, _ := delegationIdentity(t, now, 35, 36)
	valid := DelegationRequest{
		DelegatorPrincipal: delegator, ApplicationPrincipal: application, NodePrincipal: node,
		Actions:   []string{"application.content.get"},
		Scope:     sdkidentity.ResourceScope{Kind: sdkidentity.ScopePrincipalOwned, Owner: mustClientResourceOwner(t, delegator)},
		NotBefore: now, NotAfter: now.Add(15 * time.Minute),
	}
	cases := map[string]func(*DelegationRequest){
		"same Principal":   func(r *DelegationRequest) { r.ApplicationPrincipal = r.DelegatorPrincipal },
		"padded Principal": func(r *DelegationRequest) { r.NodePrincipal = " " + r.NodePrincipal },
		"unknown action":   func(r *DelegationRequest) { r.Actions = []string{"application.content.unknown"} },
		"duplicate action": func(r *DelegationRequest) { r.Actions = []string{r.Actions[0], r.Actions[0]} },
		"overlong validity": func(r *DelegationRequest) {
			r.NotAfter = r.NotBefore.Add(identitycontract.MaxDelegationLifetime + time.Second)
		},
		"noncanonical time": func(r *DelegationRequest) { r.NotBefore = r.NotBefore.Add(time.Nanosecond) },
		"cross-Node exact": func(r *DelegationRequest) {
			r.Scope = sdkidentity.ResourceScope{Kind: sdkidentity.ScopeExact, Resource: sdkidentity.ResourceRef{
				Node: application, Owner: mustClientResourceOwner(t, delegator), Kind: "owned-content", CanonicalID: "blob-reference",
			}}
		},
		"consent text injection": func(r *DelegationRequest) {
			r.Scope = sdkidentity.ResourceScope{Kind: sdkidentity.ScopeExact, Resource: sdkidentity.ResourceRef{
				Node: node, Owner: mustClientResourceOwner(t, delegator), Kind: "owned-content", CanonicalID: "blob\nforged-consent",
			}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			request := valid
			request.Actions = append([]string(nil), valid.Actions...)
			mutate(&request)
			_, err := NewDelegationProposal(request)
			require.Error(t, err)
		})
	}
}

func TestDelegationProposalQuotesExactScopeFieldsWithoutConsentAmbiguity(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	delegator, _, _ := delegationIdentity(t, now, 37, 38)
	application, _, _ := delegationIdentity(t, now, 39, 40)
	node, _, _ := delegationIdentity(t, now, 41, 42)
	canonicalID := `blob),owner=p1_spoof,id="quoted"`
	proposal, err := NewDelegationProposal(DelegationRequest{
		DelegatorPrincipal: delegator, ApplicationPrincipal: application, NodePrincipal: node,
		Actions: []string{"application.content.get"},
		Scope: sdkidentity.ResourceScope{Kind: sdkidentity.ScopeExact, Resource: sdkidentity.ResourceRef{
			Node: node, Owner: mustClientResourceOwner(t, delegator), Kind: "owned-content", CanonicalID: canonicalID,
		}},
		NotBefore: now, NotAfter: now.Add(15 * time.Minute),
	})
	require.NoError(t, err)
	require.Contains(t, proposal.ConsentText(), `id="blob),owner=p1_spoof,id=\"quoted\""`)
	require.NotContains(t, proposal.ConsentText(), `id=blob),owner=p1_spoof`)
}

func TestCreateDelegationRejectsSignerMismatchAndRedactsSignerFailure(t *testing.T) {
	now := time.Unix(1_900_000_000, 0).UTC()
	delegator, credential, device := delegationIdentity(t, now, 41, 42)
	application, _, _ := delegationIdentity(t, now, 43, 44)
	otherApplication, _, _ := delegationIdentity(t, now, 45, 46)
	node, _, _ := delegationIdentity(t, now, 47, 48)
	proposal, err := NewDelegationProposal(DelegationRequest{
		DelegatorPrincipal: delegator, ApplicationPrincipal: application, NodePrincipal: node,
		Actions:   []string{"application.content.get"},
		Scope:     sdkidentity.ResourceScope{Kind: sdkidentity.ScopePrincipalOwned, Owner: mustClientResourceOwner(t, delegator)},
		NotBefore: now, NotAfter: now.Add(15 * time.Minute),
	})
	require.NoError(t, err)

	_, err = CreateDelegation(context.Background(), proposal, delegationSignerFunc(func(_ context.Context, received DelegationProposal) (*sdkidentity.Artifact, error) {
		spec := received.Spec(credential)
		spec.Delegatee = otherApplication
		return sdkidentity.SignDelegation(spec, device, now)
	}))
	require.ErrorContains(t, err, "signed Delegation does not match consent")
	require.NotContains(t, err.Error(), otherApplication)

	const secret = "private key at C:/protected/alice-device.key"
	_, err = CreateDelegation(context.Background(), proposal, delegationSignerFunc(func(context.Context, DelegationProposal) (*sdkidentity.Artifact, error) {
		return nil, errors.New(secret)
	}))
	require.ErrorContains(t, err, "Delegation signer is unavailable")
	require.NotContains(t, err.Error(), secret)
}

func TestClientConfigAcceptsOpaqueDelegationWithoutDelegationSigningOracle(t *testing.T) {
	configType := reflect.TypeOf(Config{})
	field, present := configType.FieldByName("Delegation")
	require.True(t, present)
	require.Equal(t, reflect.TypeOf((*sdkidentity.Artifact)(nil)), field.Type)

	delegationSignerType := reflect.TypeOf((*DelegationSigner)(nil)).Elem()
	for index := 0; index < configType.NumField(); index++ {
		require.NotEqual(t, delegationSignerType, configType.Field(index).Type)
	}
}

func delegationIdentity(t *testing.T, now time.Time, rootByte, deviceByte byte) (string, *sdkidentity.Artifact, ed25519.PrivateKey) {
	t.Helper()
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{rootByte}, ed25519.SeedSize))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{deviceByte}, ed25519.SeedSize))
	principal := delegationDigestID("p1_", "ardents:principal:v1\x00", root.Public().(ed25519.PublicKey))
	credential, err := sdkidentity.SignKeyCredential(sdkidentity.KeyCredentialSpec{
		Subject: principal, RootPublicKey: root.Public().(ed25519.PublicKey),
		DeviceID:        delegationDigestID("d1_", "ardents:device:v1\x00", device.Public().(ed25519.PublicKey)),
		DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []sdkidentity.CredentialPurpose{sdkidentity.PurposeAuthenticate},
		NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour),
	}, root)
	require.NoError(t, err)
	return principal, credential, device
}

func delegationDigestID(prefix, domain string, material []byte) string {
	payload := append(append([]byte(domain), byte(1)), material...)
	sum := sha256.Sum256(payload)
	return prefix + strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:]))
}

func mustClientResourceOwner(t *testing.T, value string) sdkidentity.ResourceOwner {
	t.Helper()
	owner, err := sdkidentity.PrincipalOwner(value)
	require.NoError(t, err)
	return owner
}
