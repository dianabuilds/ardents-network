//go:build ignore

// Disposable R-152 protocol-composition probe; no Ardents runtime package.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/cloudflare/circl/blindsign/blindrsa"
)

type algorithm struct {
	ID         asn1.ObjectIdentifier
	Parameters asn1.RawValue
}
type pssParameters struct {
	Hash       algorithm `asn1:"explicit,tag:0"`
	Mask       algorithm `asn1:"explicit,tag:1"`
	SaltLength int       `asn1:"explicit,tag:2"`
}
type publicInfo struct {
	Algorithm algorithm
	Key       asn1.BitString
}

func pssKey(pk *rsa.PublicKey) ([]byte, error) {
	null := asn1.RawValue{Tag: 5}
	hash := algorithm{asn1.ObjectIdentifier{2, 16, 840, 1, 101, 3, 4, 2, 2}, null}
	hashDER, err := asn1.Marshal(hash)
	if err != nil {
		return nil, err
	}
	params, err := asn1.Marshal(pssParameters{
		hash, algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 8}, asn1.RawValue{FullBytes: hashDER}}, 48,
	})
	if err != nil {
		return nil, err
	}
	pkDER := x509.MarshalPKCS1PublicKey(pk)
	return asn1.Marshal(publicInfo{
		algorithm{asn1.ObjectIdentifier{1, 2, 840, 113549, 1, 1, 10}, asn1.RawValue{FullBytes: params}},
		asn1.BitString{Bytes: pkDER, BitLength: 8 * len(pkDER)},
	})
}

func challenge(receiver string) []byte {
	issuer := []byte("issuer.example.invalid")
	origin := []byte(receiver)
	context := sha256.Sum256([]byte("ardents-admission-context-v1\x00network\x00duty\x00forward\x001"))
	out := binary.BigEndian.AppendUint16(nil, 2)
	out = binary.BigEndian.AppendUint16(out, uint16(len(issuer)))
	out = append(out, issuer...)
	out = append(out, 32)
	out = append(out, context[:]...)
	out = binary.BigEndian.AppendUint16(out, uint16(len(origin)))
	return append(out, origin...)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	if err = key.Validate(); err != nil {
		return err
	}
	encoded, err := pssKey(&key.PublicKey)
	if err != nil {
		return err
	}
	keyID := sha256.Sum256(encoded)
	generic, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		return err
	}
	genericID := sha256.Sum256(generic)
	if keyID == genericID {
		return fmt.Errorf("SPKI encodings unexpectedly agree")
	}
	client, err := blindrsa.NewClient(blindrsa.SHA384PSSDeterministic, &key.PublicKey)
	if err != nil {
		return err
	}
	signer := blindrsa.NewSigner(key)
	digest := sha256.Sum256(challenge("receiver-a.example.invalid"))
	other := sha256.Sum256(challenge("receiver-b.example.invalid"))
	rejected := 0
	elapsed := make([]int64, 0, 64)
	var tokenBytes, requestBytes int
	for i := 0; i < 64; i++ {
		start := time.Now()
		input := binary.BigEndian.AppendUint16(nil, 2)
		nonce := make([]byte, 32)
		if _, err = rand.Read(nonce); err != nil {
			return err
		}
		input = append(input, nonce...)
		input = append(input, digest[:]...)
		input = append(input, keyID[:]...)
		prepared, e := client.Prepare(rand.Reader, input)
		if e != nil {
			return e
		}
		blinded, state, e := client.Blind(rand.Reader, prepared)
		if e != nil {
			return e
		}
		signed, e := signer.BlindSign(blinded)
		if e != nil {
			return e
		}
		signature, e := client.Finalize(state, signed)
		if e != nil {
			return e
		}
		h := sha512.Sum384(input)
		if e = rsa.VerifyPSS(&key.PublicKey, crypto.SHA384, h[:], signature, &rsa.PSSOptions{SaltLength: 48, Hash: crypto.SHA384}); e != nil {
			return e
		}
		if e = client.Verify(input, signature); e != nil {
			return e
		}
		altered := append([]byte{}, input...)
		altered[3] ^= 1
		if client.Verify(altered, signature) == nil {
			return fmt.Errorf("changed input accepted")
		}
		rejected++
		altered = append([]byte{}, input...)
		copy(altered[34:66], other[:])
		if client.Verify(altered, signature) == nil {
			return fmt.Errorf("wrong receiver challenge accepted")
		}
		rejected++
		wrongSignature := append([]byte{}, signature...)
		wrongSignature[0] ^= 1
		if client.Verify(input, wrongSignature) == nil {
			return fmt.Errorf("changed signature accepted")
		}
		rejected++
		tokenBytes = len(input) + len(signature)
		requestBytes = 3 + len(blinded)
		elapsed = append(elapsed, time.Since(start).Microseconds())
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"variant": "RSABSSA-SHA384-PSS-Deterministic", "bits": key.N.BitLen(), "exponent": key.E,
		"spki_bytes": len(encoded), "spki_der_hex": hex.EncodeToString(encoded),
		"generic_spki_bytes": len(generic), "spki_ids_differ": true, "token_bytes": tokenBytes,
		"token_request_bytes": requestBytes, "roundtrips": 64, "rejections": rejected,
		"roundtrip_microseconds": elapsed, "network_test": false, "anonymity_test": false,
	})
}
