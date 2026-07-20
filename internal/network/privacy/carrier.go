package privacy

import "context"

type Carrier interface {
	PublishPrivateEnvelope(context.Context, SealedEnvelope) error
	FetchPrivateEnvelopes(context.Context, []string, string) ([]SealedEnvelope, error)
}

type LiveCarrier interface {
	PublishPrivateEnvelope(context.Context, SealedEnvelope) error
	SubscribePrivateEnvelopes(context.Context, string) (<-chan SealedEnvelope, error)
}
