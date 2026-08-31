//go:build referencec2

package main

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"errors"

	"github.com/dianabuilds/ardents-network/internal/naming/alpha"
)

func (input config) alphaCorpusAuthority() (ed25519.PublicKey, error) {
	authority, err := hex.DecodeString(input.AlphaCorpusAuthority)
	if err != nil || len(authority) != ed25519.PublicKeySize {
		return nil, errors.New("C2 fixture alpha corpus authority is invalid")
	}
	return ed25519.PublicKey(authority), nil
}

func (input config) alphaCorpusSigner() (ed25519.PublicKey, ed25519.PrivateKey, error) {
	authority, err := input.alphaCorpusAuthority()
	if err != nil {
		return nil, nil, err
	}
	private, err := base64.RawStdEncoding.DecodeString(input.AlphaCorpusPrivate)
	if err != nil || len(private) != ed25519.PrivateKeySize {
		return nil, nil, errors.New("C2 fixture alpha corpus signer is invalid")
	}
	signer := ed25519.PrivateKey(private)
	public, ok := signer.Public().(ed25519.PublicKey)
	if !ok || !bytes.Equal(authority, public) {
		return nil, nil, errors.New("C2 fixture alpha corpus signer does not match its authority")
	}
	return authority, signer, nil
}

// openAlphaCorpusFloor opens only the locally retained signed corpus state.
// The C2 User intentionally does not read alpha corpus bytes or authority
// from the Publisher publication envelope.
func (input config) openAlphaCorpusFloor(network [32]byte) (*alpha.PersistentFloor, error) {
	return input.openAlphaCorpusFloorAt(input.AlphaCorpusFloorRoot, network)
}

// openAlphaCorpusFloorAt opens one separately accepted, Endpoint-owned alpha
// corpus floor. Its caller supplies a distinct root for each fixture Endpoint.
func (input config) openAlphaCorpusFloorAt(root string, network [32]byte) (*alpha.PersistentFloor, error) {
	authority, err := input.alphaCorpusAuthority()
	if err != nil {
		return nil, err
	}
	return alpha.OpenPersistentFloor(alpha.PersistentFloorConfig{Root: root, Authority: authority,
		Cohort: "reference-c2", Network: network})
}
