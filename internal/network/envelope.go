package network

import "time"

const DefaultPubsubTopic = "/waku/2/default-waku/proto"

type Envelope struct {
	PubsubTopic  string
	ContentTopic string
	Payload      []byte
	Timestamp    time.Time
}

type DiscoveryPublishError struct {
	Published int
	Err       error
}

func (e *DiscoveryPublishError) Error() string {
	if e == nil || e.Err == nil {
		return "discovery publish failed"
	}
	return e.Err.Error()
}

func (e *DiscoveryPublishError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
