package state

func snapshotFor(value epochEnvelope) epochVerificationSnapshot {
	return epochVerificationSnapshot{
		Generation:     value.digestString(),
		NetworkID:      value.networkID,
		Epoch:          value.number,
		Digest:         value.digest,
		PreviousDigest: value.previous,
		EpochValidFrom: value.validFrom,
		ValidUntil:     value.validUntil,
		Profile:        value.profile,
		ViewRoot:       value.viewRoot,
		ViewLength:     value.viewLength,
		RejectedRoot:   value.rejectedRoot,
		RejectedLength: value.rejectedLength,
	}
}

func (value epochEnvelope) digestString() string {
	const hexadecimal = "0123456789abcdef"
	encoded := make([]byte, len(value.digest)*2)
	for index, current := range value.digest {
		encoded[index*2] = hexadecimal[current>>4]
		encoded[index*2+1] = hexadecimal[current&15]
	}
	return string(encoded)
}
