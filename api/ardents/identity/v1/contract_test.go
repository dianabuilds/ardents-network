package identitycontract

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestApplicationEnrollmentTicketTextCodecIsCanonical(t *testing.T) {
	raw := bytes.Repeat([]byte{0xa7}, ApplicationEnrollmentTicketBytes)
	encoded, ok := EncodeApplicationEnrollmentTicket(raw)
	require.True(t, ok)
	decoded, ok := DecodeApplicationEnrollmentTicket(encoded)
	require.True(t, ok)
	require.Equal(t, raw, decoded[:])

	for _, invalid := range []string{"", " " + encoded, encoded + "\n", encoded + "="} {
		_, ok = DecodeApplicationEnrollmentTicket(invalid)
		require.False(t, ok)
	}
	_, ok = EncodeApplicationEnrollmentTicket(make([]byte, ApplicationEnrollmentTicketBytes))
	require.False(t, ok)
}

func TestVersionOneNumericalBoundaries(t *testing.T) {
	if ChallengeIDBytes != 16 || ChallengeNonceBytes != 32 || PeerBindingBytes != 32 || SessionSecretBytes != 32 || ChallengeLifetime != 120*time.Second {
		t.Fatal("challenge shape boundary")
	}
	if MaxActiveChallenges != 4096 || MaxActiveChallengesPerSource != 8 || BeginRatePerMinute != 10 || BeginRateBurst != 8 {
		t.Fatal("challenge capacity boundary")
	}
	if DefaultSessionLifetime != 15*time.Minute || MaxSessionLifetime != time.Hour || MaxActiveSessions != 16384 || MaxActiveSessionsPerSourceKey != 16 {
		t.Fatal("session boundary")
	}
	if !ValidActionCount(64) || ValidActionCount(65) {
		t.Fatal("action count boundary")
	}
	if !ValidActionSyntax(strings.Repeat("a", 128)) || ValidActionSyntax(strings.Repeat("a", 129)) {
		t.Fatal("action length boundary")
	}
	if !ValidResourceKindSyntax(strings.Repeat("k", 32)) || ValidResourceKindSyntax(strings.Repeat("k", 33)) {
		t.Fatal("resource kind boundary")
	}
	if !ValidCanonicalResourceIDSize(512) || ValidCanonicalResourceIDSize(513) {
		t.Fatal("canonical resource ID boundary")
	}
	if !ValidKeyCredentialSize(4<<10) || ValidKeyCredentialSize((4<<10)+1) {
		t.Fatal("Credential size boundary")
	}
	if !ValidArtifactSize(16<<10) || ValidArtifactSize((16<<10)+1) {
		t.Fatal("artifact size boundary")
	}
	if !ValidApplicationDiscoverySchemeCount(1) || !ValidApplicationDiscoverySchemeCount(3) ||
		ValidApplicationDiscoverySchemeCount(0) || ValidApplicationDiscoverySchemeCount(4) {
		t.Fatal("Application Discovery scheme count boundary")
	}
	if !ValidApplicationDiscoveryTargetCount(1) || !ValidApplicationDiscoveryTargetCount(8) ||
		ValidApplicationDiscoveryTargetCount(0) || ValidApplicationDiscoveryTargetCount(9) {
		t.Fatal("Application Discovery target count boundary")
	}
	for _, scheme := range []string{
		ApplicationDiscoverySchemeHTTPS, ApplicationDiscoverySchemeHTTP, ApplicationDiscoverySchemeTCP,
	} {
		if !IsApplicationDiscoveryScheme(scheme) {
			t.Fatal("Application Discovery direct scheme catalogue")
		}
	}
	if IsApplicationDiscoveryScheme("waku") {
		t.Fatal("Application Discovery unknown scheme")
	}
	if time.Unix(LowerTimestampUnix, 0).UTC() != time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatal("lower timestamp")
	}
	if time.Unix(UpperTimestampUnix, 0).UTC() != time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC) {
		t.Fatal("upper timestamp")
	}
}

func TestOperatorNodeFeaturesActionHasNoCapabilitiesAlias(t *testing.T) {
	require.True(t, IsRegisteredAction(InterfaceOperator, "node.features"))
	require.False(t, IsRegisteredAction(InterfaceOperator, "node.capabilities"))
	require.False(t, IsRegisteredAction(InterfaceApplication, "node.features"))
}

func TestApplicationActionContractsOwnMutationClassification(t *testing.T) {
	put, ok := LookupApplicationAction("application.content.put")
	require.True(t, ok)
	require.True(t, put.Mutating)

	get, ok := LookupApplicationAction("application.content.get")
	require.True(t, ok)
	require.False(t, get.Mutating)

	_, ok = LookupApplicationAction("application.content.unknown")
	require.False(t, ok)
}

func TestApplicationDiscoveryIdentityContractsAreExactAndOwnerless(t *testing.T) {
	resolve, ok := LookupApplicationAction("application.discovery.resolve")
	require.True(t, ok)
	require.False(t, resolve.Mutating)
	require.True(t, IsRegisteredAction(InterfaceApplication, "application.discovery.resolve"))
	require.False(t, IsRegisteredAction(InterfaceOperator, "application.discovery.resolve"))

	serviceType, ok := LookupResourceKind("service-type")
	require.True(t, ok)
	require.False(t, serviceType.AllowEmptyID)
	require.False(t, serviceType.OwnerRequired)
}

func TestRealmChannelDeliveryIdentityContractIsOperatorOnlyAndOwnerless(t *testing.T) {
	for _, action := range []string{
		"realm.channel.delivery.prepare",
		"realm.channel.delivery.issue",
		"realm.channel.delivery.install",
		"realm.channel.delivery.acknowledge",
	} {
		require.True(t, IsRegisteredAction(InterfaceOperator, action))
		require.False(t, IsRegisteredAction(InterfaceApplication, action))
	}
	delivery, ok := LookupResourceKind("realm-channel-delivery")
	require.True(t, ok)
	require.False(t, delivery.AllowEmptyID)
	require.False(t, delivery.OwnerRequired)
}
