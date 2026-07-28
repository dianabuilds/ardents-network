package capability

import (
	"bytes"
	"crypto/ecdh"
	"crypto/ed25519"
	"crypto/hpke"
	"encoding/hex"
	"path/filepath"
	"testing"
	"time"

	identityapi "ardents/internal/identity"

	"github.com/stretchr/testify/require"
)

func TestMemberAtomicallyInstallsInitialGenerationAndReplaysStableReceipt(t *testing.T) {
	now := capabilityTestNow
	subjectGrant, issuerPublic, issuerPrivate := signedTestGrant(t, 1)
	storePath := filepath.Join(t.TempDir(), "member-capabilities.db")
	storeKey := bytes.Repeat([]byte{0x91}, 32)
	trust := trustedIssuer(issuerPublic)
	senderGrant := subjectGrant
	senderGrant.GrantID = fixedID(0x61)
	senderGrant.SubjectPrincipal = otherPrincipal()
	senderGrant, err := SignGrant(senderGrant, issuerPrivate)
	require.NoError(t, err)
	member, err := NewService(
		storePath, storeKey,
		subjectGrant.SubjectPrincipal,
		trust,
		allowCapabilityAdmission{},
		func() time.Time { return now },
	)
	require.NoError(t, err)
	attestation, err := member.AttestDeliveryPublicKey(
		subjectIdentityPrivate(t, subjectGrant.SubjectPrincipal),
		now.Add(time.Hour),
	)
	require.NoError(t, err)
	bundle := GenerationBundle{
		Version:            1,
		RealmID:            "r1_00112233445566778899aabbccddeeff",
		AuthorityPrincipal: subjectGrant.IssuerPrincipal,
		AuthorityEpoch:     1,
		AuthoritySequence:  2,
		OperationID:        "rao1_00112233445566778899aabbccddeeff",
		DeliveryID:         "rad1_00112233445566778899aabbccddeeff",
		ChannelID:          subjectGrant.ChannelID,
		ChannelClass:       subjectGrant.Scope,
		Generation:         1,
		RecipientPrincipal: subjectGrant.SubjectPrincipal,
		DeliveryKeyDigest:  DeliveryPublicKeyDigest(attestation.DeliveryPublicKey),
		SubjectGrant:       subjectGrant,
		SenderGrants:       []identityapi.CapabilityGrant{subjectGrant, senderGrant},
		Revocations:        []identityapi.CapabilityRevocation{},
		ActivationPhase:    DeliveryPhaseInstalled,
		DrainDeadline:      now,
		ExpiresAt:          now.Add(time.Hour),
		ReceiptKey:         bytes.Repeat([]byte{0x92}, 32),
	}
	sealed, err := SealGenerationBundleForRecipient(
		bundle, attestation, now,
		func(message []byte) ([]byte, error) {
			return ed25519.Sign(issuerPrivate, message), nil
		},
	)
	require.NoError(t, err)
	require.NotContains(t, sealed.Envelope, subjectGrant.Secret.Bytes())

	wrongKey, err := NewService(
		filepath.Join(t.TempDir(), "wrong-key-member.db"), bytes.Repeat([]byte{0x93}, 32),
		subjectGrant.SubjectPrincipal, trust, allowCapabilityAdmission{}, func() time.Time { return now },
	)
	require.NoError(t, err)
	_, err = wrongKey.InstallGenerationDelivery(sealed)
	require.Error(t, err)

	tamperedBeforeInstall := sealed
	tamperedBeforeInstall.Envelope = append([]byte(nil), sealed.Envelope...)
	tamperedBeforeInstall.Envelope[len(tamperedBeforeInstall.Envelope)-1] ^= 0xff
	tamperedBeforeInstall.EnvelopeDigest = generationEnvelopeDigest(tamperedBeforeInstall.Envelope)
	_, err = member.InstallGenerationDelivery(tamperedBeforeInstall)
	require.Error(t, err)

	member.installAfterCommit = func() error { return assertCrashAfterCommit }
	_, err = member.InstallGenerationDelivery(sealed)
	require.Error(t, err)
	restarted, err := NewService(
		storePath, storeKey, subjectGrant.SubjectPrincipal, trust,
		allowCapabilityAdmission{}, func() time.Time { return now.Add(30 * time.Minute) },
	)
	require.NoError(t, err)
	receipt, err := restarted.InstallGenerationDelivery(sealed)
	require.NoError(t, err)
	require.Equal(t, DeliveryPhaseInstalled, receipt.Phase)
	require.Equal(t, bundle.DeliveryID, receipt.DeliveryID)
	require.NotEmpty(t, receipt.MAC)

	replayed, err := restarted.InstallGenerationDelivery(sealed)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	reopened, err := NewService(
		storePath, storeKey, subjectGrant.SubjectPrincipal, trust,
		allowCapabilityAdmission{}, func() time.Time { return now.Add(30 * time.Minute) },
	)
	require.NoError(t, err)
	afterRestart, err := reopened.InstallGenerationDelivery(sealed)
	require.NoError(t, err)
	require.Equal(t, receipt, afterRestart)
	bindingReplay := sealed
	bindingReplay.Binding.Generation++
	_, err = reopened.InstallGenerationDelivery(bindingReplay)
	require.Error(t, err)
	expiryReplay := sealed
	expiryReplay.Binding.ExpiresAt = expiryReplay.Binding.ExpiresAt.Add(time.Second)
	_, err = reopened.InstallGenerationDelivery(expiryReplay)
	require.Error(t, err)
	resolved, err := reopened.ResolveCapability(validUse(reopened.reference(subjectGrant), subjectGrant))
	require.NoError(t, err)
	require.Equal(t, subjectGrant.Secret.Bytes(), resolved.Secret.Bytes())
	require.NoError(t, reopened.AuthorizeCapabilitySender(identityapi.CapabilitySenderUse{
		GrantID: senderGrant.GrantID, ChannelID: senderGrant.ChannelID,
		Generation: senderGrant.Generation, Subject: senderGrant.SubjectPrincipal,
		Permission: identityapi.CapabilityPublish, Scope: senderGrant.Scope,
		At: now, ObservedAt: now,
	}))

	tampered := sealed
	tampered.Envelope = append([]byte(nil), sealed.Envelope...)
	tampered.Envelope[len(tampered.Envelope)-1] ^= 0xff
	_, err = reopened.InstallGenerationDelivery(tampered)
	require.Error(t, err)
	again, err := reopened.InstallGenerationDelivery(sealed)
	require.NoError(t, err)
	require.Equal(t, receipt, again)
	expiredReplay, err := NewService(
		storePath, storeKey, subjectGrant.SubjectPrincipal, trust,
		allowCapabilityAdmission{}, func() time.Time { return now.Add(2 * time.Hour) },
	)
	require.NoError(t, err)
	_, err = expiredReplay.InstallGenerationDelivery(sealed)
	requireCapabilityCode(t, err, CodeExpired)

	forged := receipt
	forged.Phase = DeliveryPhaseActive
	forged.MAC = generationReceiptMAC(forged, bundle.ReceiptKey)
	require.NoError(t, VerifyGenerationDeliveryReceipt(forged, bundle.ReceiptKey),
		"a receipt-key holder can forge phase assertions; this is possession evidence, not persistence proof")

	expiringNow := now
	expiring, err := NewService(
		filepath.Join(t.TempDir(), "expiring-member.db"), bytes.Repeat([]byte{0x94}, 32),
		subjectGrant.SubjectPrincipal, trust, allowCapabilityAdmission{}, func() time.Time { return expiringNow },
	)
	require.NoError(t, err)
	expiringAttestation, err := expiring.AttestDeliveryPublicKey(
		subjectIdentityPrivate(t, subjectGrant.SubjectPrincipal), now.Add(time.Hour),
	)
	require.NoError(t, err)
	expiringBundle := bundle
	expiringBundle.OperationID = "rao1_10112233445566778899aabbccddeeff"
	expiringBundle.DeliveryID = "rad1_10112233445566778899aabbccddeeff"
	expiringBundle.DeliveryKeyDigest = DeliveryPublicKeyDigest(expiringAttestation.DeliveryPublicKey)
	expiringSealed, err := SealGenerationBundleForRecipient(
		expiringBundle, expiringAttestation, now,
		func(message []byte) ([]byte, error) { return ed25519.Sign(issuerPrivate, message), nil },
	)
	require.NoError(t, err)
	expiringNow = now.Add(time.Hour)
	_, err = expiring.InstallGenerationDelivery(expiringSealed)
	requireCapabilityCode(t, err, CodeExpired)
}

var assertCrashAfterCommit = &testCrashError{}

type testCrashError struct{}

func (*testCrashError) Error() string { return "simulated crash after member commit" }

func TestGenerationDeliveryHPKESuiteMatchesRFC9180AppendixA2(t *testing.T) {
	decode := func(value string) []byte {
		raw, err := hex.DecodeString(value)
		require.NoError(t, err)
		return raw
	}
	kem := hpke.DHKEM(ecdh.X25519())
	private, err := kem.NewPrivateKey(decode(
		"8057991eef8f1f1af18f4a9491d16a1ce333f695d4db8e38da75975c4478e0fb",
	))
	require.NoError(t, err)
	require.Equal(t, decode(
		"4310ee97d88cc1f088a5576c77ab0cf5c3ac797f3d95139c6c84b5429c59662a",
	), private.PublicKey().Bytes())
	recipient, err := hpke.NewRecipient(
		decode("1afa08d3dec047a643885163f1180476fa7ddb54c6a8029ea33f95796bf2ac4a"),
		private, hpke.HKDFSHA256(), hpke.ChaCha20Poly1305(),
		decode("4f6465206f6e2061204772656369616e2055726e"),
	)
	require.NoError(t, err)
	plain, err := recipient.Open(
		decode("436f756e742d30"),
		decode("1c5250d8034ec2b784ba2cfd69dbdb8af406cfe3ff938e131f0def8c8b60b4db21993c62ce81883d2dd1b51a28"),
	)
	require.NoError(t, err)
	require.Equal(t, decode("4265617574792069732074727574682c20747275746820626561757479"), plain)
}
