package stock

import "time"

// ExchangeBinding is the opaque application-selected delivery identity.
// It selects no route and has no transport capabilities.
type ExchangeBinding struct{ ID, ProfileDigest [32]byte }

// Presentation binds token consumption to the intended receiver and operation.
type Presentation struct {
	NetworkID, StateGeneration, StateDigest, ProfileDigest [32]byte
	RecipientNodeID, ChannelNonce                          [32]byte
	RecipientDutyGeneration                                uint64
	Deadline                                               time.Time
}
