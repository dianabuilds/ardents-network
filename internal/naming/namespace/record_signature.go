package namespace

import (
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"errors"
)

const (
	signedRecordSchema        uint16 = 1
	maximumRecordPayloadBytes        = 1_846
	maximumSignedRecordBytes         = maximumRecordPayloadBytes + 2 + 8 + ed25519.SignatureSize
	recordDomain                     = "ardents-name-record-v1"
)

// SignRecord signs one canonical Record for exactly one network and returns a
// strict self-contained container. The Record Authority must be the lowercase
// hexadecimal public half of private.
func SignRecord(network [32]byte, record Record, private ed25519.PrivateKey) ([]byte, error) {
	if network == [32]byte{} || len(private) != ed25519.PrivateKeySize {
		return nil, errors.New("name record signing identity is invalid")
	}
	public := private[ed25519.SeedSize:]
	if record.Authority != hex.EncodeToString(public) {
		return nil, errors.New("name record Authority does not match signer")
	}
	recordWire, err := EncodeRecord(record)
	if err != nil || len(recordWire) > maximumRecordPayloadBytes {
		return nil, errors.New("name record cannot be signed")
	}
	signature := ed25519.Sign(private, recordTranscript(network, recordWire))
	if !ed25519.Verify(ed25519.PublicKey(public), recordTranscript(network, recordWire), signature) {
		return nil, errors.New("name Authority private key is inconsistent")
	}
	out := make([]byte, 0, 2+8+len(recordWire)+len(signature))
	out = binary.BigEndian.AppendUint16(out, signedRecordSchema)
	out = binary.BigEndian.AppendUint64(out, uint64(len(recordWire)))
	out = append(out, recordWire...)
	out = append(out, signature...)
	return out, nil
}

// VerifyRecord authenticates and decodes one exact signed Record container.
func VerifyRecord(network [32]byte, signed []byte) (Record, error) {
	if network == [32]byte{} || len(signed) < 2+8+ed25519.SignatureSize || len(signed) > maximumSignedRecordBytes {
		return Record{}, errors.New("signed name record is malformed")
	}
	if binary.BigEndian.Uint16(signed[:2]) != signedRecordSchema {
		return Record{}, errors.New("signed name record schema is invalid")
	}
	size := binary.BigEndian.Uint64(signed[2:10])
	if size == 0 || size > maximumRecordPayloadBytes || size != uint64(len(signed)-10-ed25519.SignatureSize) {
		return Record{}, errors.New("signed name record length is invalid")
	}
	recordWire := signed[10 : 10+int(size)]
	signature := signed[10+int(size):]
	record, err := DecodeRecord(recordWire)
	if err != nil {
		return Record{}, errors.New("signed name record contains an invalid Record")
	}
	public, err := canonicalAuthority(record.Authority)
	if err != nil || !ed25519.Verify(public, recordTranscript(network, recordWire), signature) {
		return Record{}, errors.New("name record signature is invalid")
	}
	return record, nil
}

func canonicalAuthority(raw string) (ed25519.PublicKey, error) {
	decoded, err := hex.DecodeString(raw)
	if err != nil || len(decoded) != ed25519.PublicKeySize || hex.EncodeToString(decoded) != raw {
		return nil, errors.New("name Authority is not a canonical Ed25519 key")
	}
	return ed25519.PublicKey(decoded), nil
}

func recordTranscript(network [32]byte, record []byte) []byte {
	out := make([]byte, 0, 2+len(recordDomain)+32+8+len(record))
	out = binary.BigEndian.AppendUint16(out, uint16(len(recordDomain)))
	out = append(out, recordDomain...)
	out = append(out, network[:]...)
	out = binary.BigEndian.AppendUint64(out, uint64(len(record)))
	return append(out, record...)
}
