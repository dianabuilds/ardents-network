package state

import "errors"

func readEpochView(decoder *byteDecoder, epoch *verifiedEpoch) error {
	for _, target := range []*[32]byte{&epoch.inputRoot, &epoch.viewRoot} {
		value, err := decoder.take(32)
		if err != nil {
			return err
		}
		copy(target[:], value)
	}
	var err error
	if epoch.viewLength, err = decoder.u32(); err != nil || epoch.viewLength > 64 {
		return errors.New("epoch view length is invalid")
	}
	rejected, err := decoder.take(32)
	if err != nil {
		return err
	}
	copy(epoch.rejectedRoot[:], rejected)
	if epoch.rejectedLength, err = decoder.u32(); err != nil || epoch.rejectedLength > 64 {
		return errors.New("epoch rejection length is invalid")
	}
	seed, err := decoder.take(32)
	if err != nil {
		return err
	}
	copy(epoch.assignmentSeed[:], seed)
	algorithm, err := decoder.text(64)
	if err != nil || algorithm != "ardents-h3-role-domain-v1" {
		return errors.New("epoch assignment algorithm is unsupported")
	}
	return readEpochSummaries(decoder, epoch)
}

func readEpochSummaries(decoder *byteDecoder, epoch *verifiedEpoch) error {
	var err error
	if epoch.eligibleCount, err = decoder.u32(); err != nil || epoch.eligibleCount > 64 {
		return errors.New("eligible count is invalid")
	}
	if epoch.eligibleCapacity, err = decoder.u32(); err != nil {
		return err
	}
	if epoch.familyCount, err = decoder.u16(); err != nil || epoch.familyCount > 64 {
		return errors.New("family count is invalid")
	}
	if epoch.maxFamilyCount, err = decoder.u16(); err != nil || epoch.maxFamilyCount > 64 {
		return errors.New("family concentration is invalid")
	}
	if epoch.maxFamilyCapacity, err = decoder.u32(); err != nil {
		return err
	}
	count, err := decoder.one()
	if err != nil || count == 0 || count > 16 {
		return errors.New("role domain count is invalid")
	}
	for range int(count) {
		domain, readErr := decoder.text(32)
		if readErr != nil {
			return readErr
		}
		if len(epoch.domains) > 0 && epoch.domains[len(epoch.domains)-1].id >= domain {
			return errors.New("role domain order is not canonical")
		}
		domainCount, readErr := decoder.u16()
		if readErr != nil || domainCount > 64 {
			return errors.New("role domain count summary is invalid")
		}
		capacity, readErr := decoder.u32()
		if readErr != nil {
			return readErr
		}
		epoch.domains = append(epoch.domains, domainSummary{domain, domainCount, capacity})
	}
	return nil
}
