package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

type verifiedEpoch struct {
	digest            [32]byte
	networkID         [32]byte
	number            uint64
	previous          [32]byte
	validFrom         time.Time
	validUntil        time.Time
	cutoff            uint32
	inputRoot         [32]byte
	viewRoot          [32]byte
	viewLength        uint32
	rejectedRoot      [32]byte
	rejectedLength    uint32
	assignmentSeed    [32]byte
	eligibleCount     uint32
	eligibleCapacity  uint32
	familyCount       uint16
	maxFamilyCount    uint16
	maxFamilyCapacity uint32
	domains           []domainSummary
	signatures        []epochSignature
}

type epochSignature struct {
	id        [32]byte
	signature []byte
}

type domainSummary struct {
	id       string
	count    uint16
	capacity uint32
}

func parseEpoch(raw []byte) (verifiedEpoch, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return verifiedEpoch{}, errors.New("epoch framing length is invalid")
	}
	decoder := byteDecoder{raw: raw}
	magic, err := decoder.take(4)
	if err != nil || string(magic) != "AREP" {
		return verifiedEpoch{}, errors.New("epoch magic is invalid")
	}
	version, err := decoder.one()
	if err != nil || version != 1 {
		return verifiedEpoch{}, errors.New("epoch schema version is invalid")
	}
	var epoch verifiedEpoch
	if err := readEpochCommitment(&decoder, &epoch); err != nil {
		return verifiedEpoch{}, err
	}
	unsignedEnd := decoder.offset
	count, err := decoder.one()
	if err != nil || count == 0 || count > 16 {
		return verifiedEpoch{}, errors.New("epoch signer count is invalid")
	}
	for range int(count) {
		idBytes, readErr := decoder.take(32)
		if readErr != nil {
			return verifiedEpoch{}, readErr
		}
		var signature epochSignature
		copy(signature.id[:], idBytes)
		signature.signature, readErr = decoder.take(ed25519.SignatureSize)
		if readErr != nil {
			return verifiedEpoch{}, readErr
		}
		if len(epoch.signatures) > 0 && bytes.Compare(epoch.signatures[len(epoch.signatures)-1].id[:], signature.id[:]) >= 0 {
			return verifiedEpoch{}, errors.New("epoch signer order is not canonical")
		}
		epoch.signatures = append(epoch.signatures, signature)
	}
	if decoder.offset != len(raw) {
		return verifiedEpoch{}, errors.New("epoch has trailing bytes")
	}
	epoch.digest = sha256.Sum256(raw[:unsignedEnd])
	return epoch, nil
}

func readEpochCommitment(decoder *byteDecoder, epoch *verifiedEpoch) error {
	network, err := decoder.take(32)
	if err != nil {
		return err
	}
	copy(epoch.networkID[:], network)
	if epoch.number, err = decoder.u64(); err != nil || epoch.number == 0 {
		return errors.New("epoch number is invalid")
	}
	previous, err := decoder.take(32)
	if err != nil {
		return err
	}
	copy(epoch.previous[:], previous)
	from, err := decoder.i64()
	if err != nil {
		return err
	}
	until, err := decoder.i64()
	if err != nil || until <= from {
		return errors.New("epoch validity interval is invalid")
	}
	epoch.validFrom, epoch.validUntil = time.Unix(from, 0).UTC(), time.Unix(until, 0).UTC()
	if epoch.cutoff, err = decoder.u32(); err != nil || epoch.cutoff > 64 {
		return errors.New("epoch cutoff is invalid")
	}
	profile, err := decoder.text(64)
	if err != nil || profile != "h3-role-probe-v1" {
		return errors.New("epoch profile is unsupported")
	}
	return readEpochView(decoder, epoch)
}

func verifyChain(input Case, evidence persistedEvidence) (verifiedEpoch, error) {
	seen := make(map[string]bool)
	var load func(string, bool) (verifiedEpoch, error)
	load = func(name string, tip bool) (verifiedEpoch, error) {
		if !canonicalGeneration.MatchString(name) || seen[name] || len(seen) >= 64 {
			return verifiedEpoch{}, errors.New("generation chain is cyclic or exceeds its bound")
		}
		seen[name] = true
		raw, exists := evidence.generations[name]
		if !exists {
			return verifiedEpoch{}, errors.New("generation chain member is missing")
		}
		epoch, err := verifyEpoch(input, raw, !tip)
		if err != nil || fmt.Sprintf("%x", epoch.digest) != name {
			return verifiedEpoch{}, errors.Join(errors.New("generation chain member is invalid"), err)
		}
		if epoch.number == 1 {
			if epoch.previous != [32]byte{} {
				return verifiedEpoch{}, errors.New("genesis previous digest is not zero")
			}
			return epoch, nil
		}
		prior, err := load(fmt.Sprintf("%x", epoch.previous), false)
		if err != nil || prior.number+1 != epoch.number || prior.digest != epoch.previous {
			return verifiedEpoch{}, errors.Join(errors.New("epoch transition is invalid"), err)
		}
		return epoch, nil
	}
	return load(evidence.current, true)
}

func verifyEpoch(input Case, raw []byte, historical bool) (verifiedEpoch, error) {
	epoch, err := parseEpoch(raw)
	if err != nil {
		return verifiedEpoch{}, err
	}
	if epoch.networkID != input.NetworkID {
		return verifiedEpoch{}, errors.New("epoch network identity is invalid")
	}
	now := input.Now.UTC()
	if historical {
		now = epoch.validFrom
	}
	if now.Before(epoch.validFrom) || !now.Before(epoch.validUntil) {
		return verifiedEpoch{}, errors.New("epoch is not strictly current")
	}
	for _, signature := range epoch.signatures {
		public, exists := input.Authorities[signature.id]
		if !exists || !ed25519.Verify(public, epoch.digest[:], signature.signature) {
			return verifiedEpoch{}, errors.New("epoch signer is unknown or invalid")
		}
	}
	if len(epoch.signatures) < input.Threshold {
		return verifiedEpoch{}, errors.New("epoch does not meet its signature threshold")
	}
	return epoch, nil
}
