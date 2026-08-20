//go:build ignore

package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"

	"github.com/dianabuilds/ardents-network/internal/naming"
)

const claimOrderRule = "ardents-name-claim-order-v1"

type Outcome string

const (
	Accepted         Outcome = "accepted"
	OrderedCollision Outcome = "ordered-collision"
	Conflict         Outcome = "conflict"
	Fork             Outcome = "fork"
	Unavailable      Outcome = "unavailable"
)

type Claim struct {
	Ordinal    uint32
	Name       string
	Secret     [32]byte
	Authority  [32]byte
	Commitment [32]byte
	Signature  []byte
}

type ClaimSetProof struct {
	Network       [32]byte
	Epoch         uint64
	Rule          string
	Complete      bool
	Claims        []Claim
	SetRoot       [32]byte
	SetSignatures []SetSignature
	AlternateSets []SetStatement
}

type Policy struct {
	Network      [32]byte
	Rule         string
	MinimumEpoch uint64
	MaxClaims    uint32
	Authorities  map[[32]byte]ed25519.PublicKey
	Threshold    int
}

type SetSignature struct {
	AuthorityID [32]byte
	Signature   []byte
}

type SetStatement struct {
	Root       [32]byte
	Length     uint32
	Signatures []SetSignature
}

type Result struct {
	Outcome       Outcome
	WinnerOrdinal uint32
	LoserOrdinals []uint32
}

func Verify(policy Policy, proof ClaimSetProof) (Result, error) {
	if policy.Network == [32]byte{} || policy.Rule != claimOrderRule || policy.MinimumEpoch == 0 ||
		policy.MaxClaims == 0 || policy.MaxClaims > 64 ||
		policy.Threshold < 2 || policy.Threshold > len(policy.Authorities) || len(policy.Authorities) > 16 ||
		proof.Network != policy.Network || proof.Rule == "" || proof.Epoch < policy.MinimumEpoch ||
		!proof.Complete || len(proof.Claims) == 0 || uint32(len(proof.Claims)) > policy.MaxClaims {
		return Result{Outcome: Unavailable}, errors.New("claim-set proof is incomplete")
	}
	claims := append([]Claim(nil), proof.Claims...)
	sort.Slice(claims, func(i, j int) bool { return claims[i].Ordinal < claims[j].Ordinal })
	primary := SetStatement{Root: proof.SetRoot, Length: uint32(len(proof.Claims)), Signatures: proof.SetSignatures}
	if proof.SetRoot != claimSetRoot(claims) || !validSetSignatures(policy, proof, primary) {
		return Result{Outcome: Unavailable}, errors.New("claim-set authentication is invalid")
	}
	if proof.Rule != policy.Rule {
		return Result{Outcome: Fork}, nil
	}
	if len(proof.AlternateSets) > 1 {
		return Result{Outcome: Unavailable}, errors.New("claim-set alternatives are unbounded")
	}
	for _, alternate := range proof.AlternateSets {
		if !validSetSignatures(policy, proof, alternate) {
			return Result{Outcome: Unavailable}, errors.New("alternate claim-set authentication is invalid")
		}
		if alternate.Root == primary.Root && alternate.Length == primary.Length {
			return Result{Outcome: Unavailable}, errors.New("alternate claim set duplicates the primary")
		}
		return Result{Outcome: Fork}, nil
	}
	for i := range claims {
		claim := claims[i]
		parsed, err := naming.Parse(claim.Name)
		if err != nil || string(parsed) != claim.Name || claim.Authority == [32]byte{} ||
			claim.Secret == [32]byte{} || claim.Commitment != CommitmentFor(proof.Network, proof.Epoch, claim) ||
			len(claim.Signature) != ed25519.SignatureSize ||
			!ed25519.Verify(ed25519.PublicKey(claim.Authority[:]), RevealTranscript(proof.Network, proof.Epoch, claim), claim.Signature) {
			return Result{Outcome: Conflict}, errors.New("claim is invalid")
		}
		if i > 0 && claims[i].Name != claims[0].Name {
			return Result{Outcome: Conflict}, errors.New("claim set contains multiple names")
		}
	}
	for i := 1; i < len(claims); i++ {
		if claims[i-1].Ordinal == claims[i].Ordinal {
			return Result{Outcome: Fork}, errors.New("claim ordinal is duplicated")
		}
	}
	result := Result{Outcome: Accepted, WinnerOrdinal: claims[0].Ordinal}
	for _, claim := range claims[1:] {
		result.LoserOrdinals = append(result.LoserOrdinals, claim.Ordinal)
	}
	return result, nil
}

func validSetSignatures(policy Policy, proof ClaimSetProof, statement SetStatement) bool {
	if len(statement.Signatures) < policy.Threshold || len(statement.Signatures) > len(policy.Authorities) ||
		statement.Length == 0 || statement.Length > 64 {
		return false
	}
	transcript := setTranscript(proof.Network, proof.Epoch, proof.Rule, statement.Root, statement.Length)
	for i, signed := range statement.Signatures {
		if i > 0 && bytes.Compare(statement.Signatures[i-1].AuthorityID[:], signed.AuthorityID[:]) >= 0 {
			return false
		}
		public, exists := policy.Authorities[signed.AuthorityID]
		if !exists || sha256.Sum256(public) != signed.AuthorityID || len(public) != ed25519.PublicKeySize ||
			len(signed.Signature) != ed25519.SignatureSize || !ed25519.Verify(public, transcript, signed.Signature) {
			return false
		}
	}
	return true
}

func claimSetTranscript(proof ClaimSetProof) []byte {
	return setTranscript(proof.Network, proof.Epoch, proof.Rule, proof.SetRoot, uint32(len(proof.Claims)))
}

func setTranscript(network [32]byte, epoch uint64, rule string, root [32]byte, length uint32) []byte {
	out := appendText(nil, "ardents-name-claim-set-v1")
	out = append(out, network[:]...)
	out = binary.BigEndian.AppendUint64(out, epoch)
	out = appendText(out, rule)
	out = append(out, root[:]...)
	return binary.BigEndian.AppendUint32(out, length)
}

func claimSetRoot(claims []Claim) [32]byte {
	leaves := make([][32]byte, len(claims))
	for i, claim := range claims {
		leaf := []byte{0}
		leaf = binary.BigEndian.AppendUint32(leaf, claim.Ordinal)
		leaf = append(leaf, claim.Commitment[:]...)
		leaves[i] = sha256.Sum256(leaf)
	}
	return merkleRoot(leaves)
}

func merkleRoot(leaves [][32]byte) [32]byte {
	if len(leaves) == 1 {
		return leaves[0]
	}
	split := 1
	for split<<1 < len(leaves) {
		split <<= 1
	}
	left, right := merkleRoot(leaves[:split]), merkleRoot(leaves[split:])
	node := append([]byte{1}, left[:]...)
	node = append(node, right[:]...)
	return sha256.Sum256(node)
}

func CommitmentFor(network [32]byte, epoch uint64, claim Claim) [32]byte {
	transcript := appendText(nil, "ardents-name-claim-commit-v1")
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, epoch)
	transcript = appendText(transcript, claim.Name)
	transcript = append(transcript, claim.Authority[:]...)
	transcript = append(transcript, claim.Secret[:]...)
	return sha256.Sum256(transcript)
}

func RevealTranscript(network [32]byte, epoch uint64, claim Claim) []byte {
	transcript := appendText(nil, "ardents-name-claim-reveal-v1")
	transcript = append(transcript, network[:]...)
	transcript = binary.BigEndian.AppendUint64(transcript, epoch)
	transcript = binary.BigEndian.AppendUint32(transcript, claim.Ordinal)
	transcript = append(transcript, claim.Commitment[:]...)
	return transcript
}

func appendText(out []byte, value string) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(value)))
	return append(out, value...)
}
