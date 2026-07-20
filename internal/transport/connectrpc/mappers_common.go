package connectrpc

import (
	"time"

	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func ts(t time.Time) *timestamppb.Timestamp {
	if t.IsZero() {
		return nil
	}
	return timestamppb.New(t)
}

func tsp(t *time.Time) *timestamppb.Timestamp {
	if t == nil || t.IsZero() {
		return nil
	}
	return timestamppb.New(*t)
}

func fromTS(t *timestamppb.Timestamp) time.Time {
	if t == nil {
		return time.Time{}
	}
	return t.AsTime().UTC()
}

func toStruct(in map[string]any) *structpb.Struct {
	if len(in) == 0 {
		return nil
	}
	out, err := structpb.NewStruct(in)
	if err != nil {
		return nil
	}
	return out
}

func fromStruct(in *structpb.Struct) map[string]any {
	if in == nil {
		return nil
	}
	return in.AsMap()
}

func cloneBoolMap(in map[string]bool) map[string]bool {
	if in == nil {
		return nil
	}
	out := make(map[string]bool, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
