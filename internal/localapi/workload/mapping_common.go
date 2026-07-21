package workload

import (
	"time"

	protocol "ardents/internal/localapi/protocol"

	"google.golang.org/protobuf/types/known/timestamppb"
)

func statusProto(state, reason string, accepted bool) *protocol.OperationStatus {
	return &protocol.OperationStatus{State: state, Reason: reason, Accepted: accepted}
}

func ts(value time.Time) *timestamppb.Timestamp {
	if value.IsZero() {
		return nil
	}
	return timestamppb.New(value)
}

func tsp(value *time.Time) *timestamppb.Timestamp {
	if value == nil || value.IsZero() {
		return nil
	}
	return timestamppb.New(*value)
}
