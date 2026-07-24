package messaging_test

import (
	"encoding/hex"
	"testing"

	messagingprotocol "ardents/internal/messaging/protocol"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestPrivateMessageV1WireCompatibility(t *testing.T) {
	const wireHex = "080110021a10000102030405060708090a0b0c0d0e0f220c6469643a6b65793a746573742a02aabb30033a026f6b42030102034a020000"

	wire, err := hex.DecodeString(wireHex)
	require.NoError(t, err)

	var message messagingprotocol.PrivateMessageV1
	require.NoError(t, proto.Unmarshal(wire, &message))
	require.Equal(t, uint32(1), message.ProtocolVersion)
	require.Equal(t, messagingprotocol.MessageClass_BLOB_FETCH_REQUEST, message.MessageClass)
	require.Equal(t, []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}, message.GrantId)
	require.Equal(t, "did:key:test", message.SenderPrincipal)
	require.Equal(t, []byte{0xaa, 0xbb}, message.SenderPublicKey)
	require.Equal(t, uint32(3), message.PayloadVersion)
	require.Equal(t, []byte("ok"), message.Payload)
	require.Equal(t, []byte{0x01, 0x02, 0x03}, message.Signature)
	require.Equal(t, []byte{0x00, 0x00}, message.Padding)

	roundTrip, err := proto.MarshalOptions{Deterministic: true}.Marshal(&message)
	require.NoError(t, err)
	require.Equal(t, wire, roundTrip)
}
