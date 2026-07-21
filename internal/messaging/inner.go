package messaging

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	identityprincipal "ardents/internal/identity/principal"

	"google.golang.org/protobuf/proto"
)

const signatureDomain = "ardents-private-message-signature/1"

var paddingBuckets = [...]int{1024, 4096, 16384, 65536, 131072}

func signingDigest(header envelopeHeader, message *PrivateMessageV1) ([]byte, error) {
	unsigned := proto.Clone(message).(*PrivateMessageV1)
	unsigned.Signature = nil
	unsigned.Padding = nil
	raw, err := proto.MarshalOptions{Deterministic: true}.Marshal(unsigned)
	if err != nil {
		return nil, err
	}
	header.CiphertextLength = 0
	digest := sha256.New()
	_, _ = digest.Write([]byte(signatureDomain))
	_, _ = digest.Write(header.marshal())
	_, _ = digest.Write(raw)
	return digest.Sum(nil), nil
}

func marshalPadded(message *PrivateMessageV1) ([]byte, error) {
	message.Padding = nil
	base, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil {
		return nil, err
	}
	if len(base) > maximumInnerSize {
		return nil, envelopeError(CodeEnvelopeOversized, "private message exceeds maximum inner size")
	}
	for _, bucket := range paddingBuckets {
		if len(base) > bucket {
			continue
		}
		if len(base) == bucket {
			return base, nil
		}
		paddingLength, ok := paddingForBucket(len(base), bucket)
		if !ok {
			continue
		}
		message.Padding = make([]byte, paddingLength)
		padded, marshalErr := proto.MarshalOptions{Deterministic: true}.Marshal(message)
		if marshalErr != nil {
			return nil, marshalErr
		}
		if len(padded) == bucket {
			return padded, nil
		}
	}
	return nil, envelopeError(CodeEnvelopeOversized, "private message does not fit a padding bucket")
}

func paddingForBucket(baseSize, bucket int) (int, bool) {
	for overhead := 2; overhead <= 4; overhead++ {
		paddingLength := bucket - baseSize - overhead
		if paddingLength > 0 && 1+varintSize(uint64(paddingLength)) == overhead {
			return paddingLength, true
		}
	}
	return 0, false
}

func validateDecodedInner(raw []byte, message *PrivateMessageV1) error {
	if len(message.ProtoReflect().GetUnknown()) != 0 {
		return fmt.Errorf("private message contains unknown fields")
	}
	canonical, err := proto.MarshalOptions{Deterministic: true}.Marshal(message)
	if err != nil || !bytes.Equal(canonical, raw) {
		return fmt.Errorf("private message encoding is not canonical")
	}
	if message.ProtocolVersion != 1 || message.PayloadVersion == 0 {
		return fmt.Errorf("private message version is invalid")
	}
	if len(message.GrantId) != 16 || len(message.SenderPublicKey) != ed25519.PublicKeySize ||
		len(message.Signature) != ed25519.SignatureSize {
		return fmt.Errorf("private message identity fields are invalid")
	}
	if identityprincipal.DeriveID("p", message.SenderPublicKey) != message.SenderPrincipal {
		return fmt.Errorf("private message principal does not match its public key")
	}
	if !allZero(message.Padding) {
		return fmt.Errorf("private message padding is invalid")
	}
	withoutPadding := proto.Clone(message).(*PrivateMessageV1)
	withoutPadding.Padding = nil
	unpadded, err := proto.MarshalOptions{Deterministic: true}.Marshal(withoutPadding)
	if err != nil || len(unpadded) > maximumInnerSize {
		return fmt.Errorf("private message unpadded size is invalid")
	}
	expected, ok := smallestEncodableBucket(len(unpadded))
	if !ok || len(raw) != expected {
		return fmt.Errorf("private message padding bucket is invalid")
	}
	return nil
}

func smallestEncodableBucket(size int) (int, bool) {
	for _, bucket := range paddingBuckets {
		if size == bucket {
			return bucket, true
		}
		if size < bucket {
			if _, ok := paddingForBucket(size, bucket); ok {
				return bucket, true
			}
		}
	}
	return 0, false
}

func varintSize(value uint64) int {
	var buffer [binary.MaxVarintLen64]byte
	return binary.PutUvarint(buffer[:], value)
}

func allZero(raw []byte) bool {
	return zeroBytes(raw)
}
