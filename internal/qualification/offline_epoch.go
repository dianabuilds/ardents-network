package qualification

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"time"
)

type offlineEpoch struct {
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
	domains           []independentDomain
	signatures        []offlineSignature
}

type offlineSignature struct {
	id        [32]byte
	signature []byte
}

type independentDomain struct {
	id       string
	count    uint16
	capacity uint32
}

func parseOfflineEpoch(raw []byte) (offlineEpoch, error) {
	if len(raw) == 0 || len(raw) > 1<<20 {
		return offlineEpoch{}, errors.New("epoch framing length is invalid")
	}
	d := byteDecoder{raw: raw}
	magic, err := d.take(4)
	if err != nil || string(magic) != "AREP" {
		return offlineEpoch{}, errors.New("epoch magic is invalid")
	}
	version, err := d.one()
	if err != nil || version != 1 {
		return offlineEpoch{}, errors.New("epoch schema version is invalid")
	}
	var epoch offlineEpoch
	if err := readOfflineCommitment(&d, &epoch); err != nil {
		return offlineEpoch{}, err
	}
	unsignedEnd := d.offset
	count, err := d.one()
	if err != nil || count == 0 || count > 16 {
		return offlineEpoch{}, errors.New("epoch signer count is invalid")
	}
	for range int(count) {
		idBytes, readErr := d.take(32)
		if readErr != nil {
			return offlineEpoch{}, readErr
		}
		var signature offlineSignature
		copy(signature.id[:], idBytes)
		signature.signature, readErr = d.take(ed25519.SignatureSize)
		if readErr != nil {
			return offlineEpoch{}, readErr
		}
		if len(epoch.signatures) > 0 && bytes.Compare(epoch.signatures[len(epoch.signatures)-1].id[:], signature.id[:]) >= 0 {
			return offlineEpoch{}, errors.New("epoch signer order is not canonical")
		}
		epoch.signatures = append(epoch.signatures, signature)
	}
	if d.offset != len(raw) {
		return offlineEpoch{}, errors.New("epoch has trailing bytes")
	}
	epoch.digest = sha256.Sum256(raw[:unsignedEnd])
	return epoch, nil
}

func readOfflineCommitment(d *byteDecoder, epoch *offlineEpoch) error {
	network, err := d.take(32)
	if err != nil {
		return err
	}
	copy(epoch.networkID[:], network)
	if epoch.number, err = d.u64(); err != nil || epoch.number == 0 {
		return errors.New("epoch number is invalid")
	}
	previous, err := d.take(32)
	if err != nil {
		return err
	}
	copy(epoch.previous[:], previous)
	from, err := d.i64()
	if err != nil {
		return err
	}
	until, err := d.i64()
	if err != nil || until <= from {
		return errors.New("epoch validity interval is invalid")
	}
	epoch.validFrom, epoch.validUntil = time.Unix(from, 0).UTC(), time.Unix(until, 0).UTC()
	if epoch.cutoff, err = d.u32(); err != nil || epoch.cutoff > 64 {
		return errors.New("epoch cutoff is invalid")
	}
	profile, err := d.text(64)
	if err != nil || profile != "h3-role-probe-v1" {
		return errors.New("epoch profile is unsupported")
	}
	return readOfflineView(d, epoch)
}

func readOfflineView(d *byteDecoder, epoch *offlineEpoch) error {
	for _, target := range []*[32]byte{&epoch.inputRoot, &epoch.viewRoot} {
		value, err := d.take(32)
		if err != nil {
			return err
		}
		copy(target[:], value)
	}
	var err error
	if epoch.viewLength, err = d.u32(); err != nil || epoch.viewLength > 64 {
		return errors.New("epoch view length is invalid")
	}
	rejected, err := d.take(32)
	if err != nil {
		return err
	}
	copy(epoch.rejectedRoot[:], rejected)
	if epoch.rejectedLength, err = d.u32(); err != nil || epoch.rejectedLength > 64 {
		return errors.New("epoch rejection length is invalid")
	}
	seed, err := d.take(32)
	if err != nil {
		return err
	}
	copy(epoch.assignmentSeed[:], seed)
	algorithm, err := d.text(64)
	if err != nil || algorithm != "ardents-h3-role-domain-v1" {
		return errors.New("epoch assignment algorithm is unsupported")
	}
	return readOfflineSummaries(d, epoch)
}

func readOfflineSummaries(d *byteDecoder, epoch *offlineEpoch) error {
	var err error
	if epoch.eligibleCount, err = d.u32(); err != nil || epoch.eligibleCount > 64 {
		return errors.New("eligible count is invalid")
	}
	if epoch.eligibleCapacity, err = d.u32(); err != nil {
		return err
	}
	if epoch.familyCount, err = d.u16(); err != nil || epoch.familyCount > 64 {
		return errors.New("family count is invalid")
	}
	if epoch.maxFamilyCount, err = d.u16(); err != nil || epoch.maxFamilyCount > 64 {
		return errors.New("family concentration is invalid")
	}
	if epoch.maxFamilyCapacity, err = d.u32(); err != nil {
		return err
	}
	count, err := d.one()
	if err != nil || count == 0 || count > 16 {
		return errors.New("role domain count is invalid")
	}
	for range int(count) {
		domain, readErr := d.text(32)
		if readErr != nil {
			return readErr
		}
		if len(epoch.domains) > 0 && epoch.domains[len(epoch.domains)-1].id >= domain {
			return errors.New("role domain order is not canonical")
		}
		count, readErr := d.u16()
		if readErr != nil || count > 64 {
			return errors.New("role domain count summary is invalid")
		}
		capacity, readErr := d.u32()
		if readErr != nil {
			return readErr
		}
		epoch.domains = append(epoch.domains, independentDomain{domain, count, capacity})
	}
	return nil
}

func verifyOfflineEpoch(input OfflineCase, raw []byte, historical bool) (offlineEpoch, error) {
	epoch, err := parseOfflineEpoch(raw)
	if err != nil {
		return offlineEpoch{}, err
	}
	if epoch.networkID != input.NetworkID {
		return offlineEpoch{}, errors.New("offline epoch network identity is invalid")
	}
	now := input.Now.UTC()
	if historical {
		now = epoch.validFrom
	}
	if now.Before(epoch.validFrom) || !now.Before(epoch.validUntil) {
		return offlineEpoch{}, errors.New("offline epoch is not strictly current")
	}
	valid := 0
	for _, signature := range epoch.signatures {
		public, exists := input.Authorities[signature.id]
		if !exists || !ed25519.Verify(public, epoch.digest[:], signature.signature) {
			return offlineEpoch{}, errors.New("offline epoch signer is unknown or invalid")
		}
		valid++
	}
	if valid < input.Threshold {
		return offlineEpoch{}, errors.New("offline epoch does not meet its signature threshold")
	}
	return epoch, nil
}

func keyID(public []byte) [32]byte { return sha256.Sum256(public) }
