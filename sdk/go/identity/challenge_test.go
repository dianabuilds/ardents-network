package identity

import (
	"bytes"
	"crypto/ed25519"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTypedChallengeSignersMatchServerVectors(t *testing.T) {
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x11}, 32))
	device := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x22}, 32))
	var id [16]byte
	for index := range id {
		id[index] = byte(index + 1)
	}
	var nonce, peer [32]byte
	for index := range nonce {
		nonce[index] = byte(0x20 + index)
		peer[index] = byte(0x80 + index)
	}
	challenge := Challenge{Version: 1, ID: id, Nonce: nonce, Principal: principalID(root.Public().(ed25519.PublicKey)), Binding: AuthenticationBinding{Audience: Audience{Node: principalID(ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x33}, 32)).Public().(ed25519.PublicKey)), Interface: InterfaceApplication, ProtocolMajor: 1}, TransportProfile: TransportUnixLocalV1, PeerBinding: peer}, Purpose: ChallengeSession, IssuedAt: time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC), ExpiresAt: time.Date(2030, 1, 2, 3, 6, 5, 0, time.UTC)}
	credential, err := SignKeyCredential(KeyCredentialSpec{Subject: challenge.Principal, RootPublicKey: root.Public().(ed25519.PublicKey), DeviceID: deviceID(device.Public().(ed25519.PublicKey)), DevicePublicKey: device.Public().(ed25519.PublicKey), Purposes: []CredentialPurpose{PurposeAuthenticate}, NotBefore: challenge.IssuedAt.Add(-time.Hour), NotAfter: challenge.ExpiresAt.Add(time.Hour)}, root)
	require.NoError(t, err)
	signature, err := SignAuthenticationChallenge(challenge, credential, device)
	require.NoError(t, err)
	require.Equal(t, "4e02e35c5d54243e4a6829e60f861948bf61aab5ccf39f5d40a05e96f82349895eb9e2d14c4aae8cbddd67add1cdcbacf8b8e5c264daa956c61760ab64a45d0f", hex.EncodeToString(signature))
	challenge.Purpose = ChallengeEnrollmentProof
	signature, err = SignEnrollmentChallenge(challenge, root)
	require.NoError(t, err)
	require.Equal(t, "14645769955b79f02d99fe9f528d9bc779ba81eb168dd044596f4e15ae0897e466a3bc483c8825d71c52e9e6ffcf36f4125ac4ab2143105f20e57fb023b91300", hex.EncodeToString(signature))
	_, err = SignAuthenticationChallenge(challenge, credential, device)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	challenge.Purpose = ChallengeSession
	wrong := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{0x77}, 32))
	_, err = SignAuthenticationChallenge(challenge, credential, wrong)
	require.ErrorIs(t, err, ErrInvalidChallenge)
	challenge.Purpose = ChallengeEnrollmentProof
	_, err = SignEnrollmentChallenge(challenge, wrong)
	require.ErrorIs(t, err, ErrInvalidChallenge)
}

func TestChallengeValidationBoundariesAndRedaction(t *testing.T) {
	root := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{1}, 32))
	node := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{2}, 32))
	var id [16]byte
	id[0] = 1
	var nonce, peer [32]byte
	nonce[0] = 1
	peer[0] = 1
	issued := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	challenge := Challenge{Version: 1, ID: id, Nonce: nonce, Principal: principalID(root.Public().(ed25519.PublicKey)), Binding: AuthenticationBinding{Audience: Audience{Node: principalID(node.Public().(ed25519.PublicKey)), Interface: InterfaceOperator, ProtocolMajor: 1}, TransportProfile: TransportUnixLocalV1, PeerBinding: peer}, Purpose: ChallengeSession, IssuedAt: issued, ExpiresAt: issued.Add(2 * time.Minute)}
	require.NoError(t, ValidateChallenge(challenge, issued))
	require.NoError(t, ValidateChallenge(challenge, challenge.ExpiresAt.Add(-time.Nanosecond)))
	require.ErrorIs(t, ValidateChallenge(challenge, challenge.ExpiresAt), ErrInvalidChallenge)
	challenge.ExpiresAt = challenge.ExpiresAt.Add(time.Second)
	require.ErrorIs(t, ValidateChallenge(challenge, issued), ErrInvalidChallenge)
	require.Equal(t, "identity challenge [redacted]", fmt.Sprintf("%v", challenge))
}
