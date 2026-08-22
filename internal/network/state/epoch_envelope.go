package state

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"
)

const (
	maximumEpochBytes, maximumEpochChain = 1 << 20, 64
	assignmentV1                         = "ardents-h3-role-domain-v1"
	emptyInputTag                        = byte(0x10)
	emptyViewTag                         = byte(0x11)
	emptyRejectionTag                    = byte(0x12)
)

type epochEnvelope struct {
	digest            [32]byte
	networkID         [32]byte
	number            uint64
	previous          [32]byte
	validFrom         time.Time
	validUntil        time.Time
	profile           string
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
	domains           []roleDomain
	signatures        []epochSignature
}

type epochSignature struct {
	id        [32]byte
	signature []byte
}

type roleDomain struct {
	id       string
	count    uint16
	capacity uint32
}

func parseEpoch(raw []byte) (epochEnvelope, error) {
	if len(raw) == 0 || len(raw) > maximumEpochBytes {
		return epochEnvelope{}, errors.New("epoch framing length is invalid")
	}
	d := newDecoder(raw)
	magic, err := d.bytes(4)
	if err != nil || string(magic) != "AREP" {
		return epochEnvelope{}, errors.New("epoch magic is invalid")
	}
	version, err := d.byte()
	if err != nil || version != 1 {
		return epochEnvelope{}, errors.New("epoch schema version is invalid")
	}
	var epoch epochEnvelope
	if err := decodeEpochCommitment(&d, &epoch); err != nil {
		return epochEnvelope{}, err
	}
	unsignedEnd := d.Consumed()
	signerCount, err := d.byte()
	if err != nil || signerCount == 0 || signerCount > 16 {
		return epochEnvelope{}, errors.New("epoch signer count is invalid")
	}
	for range int(signerCount) {
		idBytes, readErr := d.bytes(32)
		if readErr != nil {
			return epochEnvelope{}, readErr
		}
		var id [32]byte
		copy(id[:], idBytes)
		signature, readErr := d.bytes(ed25519.SignatureSize)
		if readErr != nil {
			return epochEnvelope{}, readErr
		}
		if len(epoch.signatures) > 0 && bytes.Compare(epoch.signatures[len(epoch.signatures)-1].id[:], id[:]) >= 0 {
			return epochEnvelope{}, errors.New("epoch signers are not in strict canonical order")
		}
		epoch.signatures = append(epoch.signatures, epochSignature{id: id, signature: signature})
	}
	if !d.done() {
		return epochEnvelope{}, errors.New("epoch contains trailing bytes")
	}
	epoch.digest = sha256.Sum256(raw[:unsignedEnd])
	return epoch, nil
}

func decodeEpochCommitment(d *decoder, epoch *epochEnvelope) error {
	network, err := d.bytes(32)
	if err != nil {
		return err
	}
	copy(epoch.networkID[:], network)
	if epoch.number, err = d.uint64(); err != nil || epoch.number == 0 {
		return errors.New("epoch number is invalid")
	}
	previous, err := d.bytes(32)
	if err != nil {
		return err
	}
	copy(epoch.previous[:], previous)
	from, err := d.int64()
	if err != nil {
		return err
	}
	until, err := d.int64()
	if err != nil || until <= from {
		return errors.New("epoch validity interval is invalid")
	}
	epoch.validFrom, epoch.validUntil = time.Unix(from, 0).UTC(), time.Unix(until, 0).UTC()
	if epoch.cutoff, err = d.uint32(); err != nil || epoch.cutoff > 64 {
		return errors.New("epoch input cutoff is invalid")
	}
	profile, err := d.text(64)
	if err != nil || !knownProfile(profile) {
		return errors.New("epoch profile is unsupported")
	}
	epoch.profile = profile
	return decodeViewCommitment(d, epoch)
}

func decodeViewCommitment(d *decoder, epoch *epochEnvelope) error {
	for _, target := range []*[32]byte{&epoch.inputRoot, &epoch.viewRoot} {
		value, err := d.bytes(32)
		if err != nil {
			return err
		}
		copy(target[:], value)
	}
	var err error
	if epoch.viewLength, err = d.uint32(); err != nil || epoch.viewLength > 64 {
		return errors.New("view length is invalid")
	}
	rejected, err := d.bytes(32)
	if err != nil {
		return err
	}
	copy(epoch.rejectedRoot[:], rejected)
	if epoch.rejectedLength, err = d.uint32(); err != nil || epoch.rejectedLength > 64 {
		return errors.New("rejection length is invalid")
	}
	seed, err := d.bytes(32)
	if err != nil {
		return err
	}
	copy(epoch.assignmentSeed[:], seed)
	algorithm, err := d.text(64)
	if err != nil || algorithm != assignmentV1 {
		return errors.New("assignment algorithm is unsupported")
	}
	return decodeSummaries(d, epoch)
}

func decodeSummaries(d *decoder, epoch *epochEnvelope) error {
	var err error
	if epoch.eligibleCount, err = d.uint32(); err != nil || epoch.eligibleCount > 64 {
		return errors.New("eligible count is invalid")
	}
	if epoch.eligibleCapacity, err = d.uint32(); err != nil {
		return err
	}
	if epoch.familyCount, err = d.uint16(); err != nil || epoch.familyCount > 64 {
		return errors.New("family count is invalid")
	}
	if epoch.maxFamilyCount, err = d.uint16(); err != nil || epoch.maxFamilyCount > 64 {
		return errors.New("family concentration is invalid")
	}
	if epoch.maxFamilyCapacity, err = d.uint32(); err != nil {
		return err
	}
	domainCount, err := d.byte()
	if err != nil || domainCount == 0 || domainCount > 16 {
		return errors.New("role domain count is invalid")
	}
	for range int(domainCount) {
		domain, readErr := d.text(32)
		if readErr != nil {
			return readErr
		}
		if len(epoch.domains) > 0 && epoch.domains[len(epoch.domains)-1].id >= domain {
			return errors.New("role domains are not in strict canonical order")
		}
		count, readErr := d.uint16()
		if readErr != nil || count > 64 {
			return errors.New("role domain count summary is invalid")
		}
		capacity, readErr := d.uint32()
		if readErr != nil {
			return readErr
		}
		epoch.domains = append(epoch.domains, roleDomain{id: domain, count: count, capacity: capacity})
	}
	return nil
}

func verifyEpoch(config epochPolicy, current *epochVerificationSnapshot, raw []byte) (epochEnvelope, error) {
	epoch, err := parseEpoch(raw)
	if err != nil {
		return epochEnvelope{}, err
	}
	if epoch.number > maximumEpochChain {
		return epochEnvelope{}, errors.New("epoch exceeds the retained chain bound")
	}
	if err := matchProfile(config.Profile, epoch.profile); err != nil {
		return epochEnvelope{}, err
	}
	if err := verifyEpochChain(current, epoch); err != nil {
		return epochEnvelope{}, err
	}
	if err := authenticateEnvelope(config, epoch, config.Now); err != nil {
		return epochEnvelope{}, err
	}
	return epoch, nil
}

func authenticateEnvelope(config epochPolicy, epoch epochEnvelope, now time.Time) error {
	if epoch.networkID != config.NetworkID {
		return errors.New("epoch network identity is wrong")
	}
	if now.Before(epoch.validFrom) || !now.Before(epoch.validUntil) {
		return errors.New("epoch is not strictly current")
	}
	valid := 0
	for _, signed := range epoch.signatures {
		public, known := config.Authorities[signed.id]
		if !known {
			return errors.New("epoch has an unknown signer")
		}
		if !ed25519.Verify(public, epoch.digest[:], signed.signature) {
			return errors.New("epoch signature is invalid")
		}
		valid++
	}
	if valid < config.Threshold {
		return fmt.Errorf("epoch has %d valid signatures, need %d", valid, config.Threshold)
	}
	return nil
}
