package content

import (
	"errors"

	contentapi "ardents/internal/content"
	applicationv1 "ardents/sdk/go/protocol/applicationv1"
	applicationv1connect "ardents/sdk/go/protocol/applicationv1/applicationv1connect"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

var (
	ErrInvalidResourceTarget = errors.New("application content resource target is invalid")
	ErrPayloadTooLarge       = errors.New("application content payload exceeds the unary limit")
)

type ResourceTarget struct {
	Kind string
	ID   string
}

func CanonicalizeResource(procedure string, message any) (ResourceTarget, error) {
	rule, err := RuleForProcedure(procedure)
	if err != nil {
		return ResourceTarget{}, err
	}

	request, ok := message.(proto.Message)
	if !ok || request == nil || !request.ProtoReflect().IsValid() || hasUnknownFields(request.ProtoReflect()) {
		return ResourceTarget{}, ErrInvalidResourceTarget
	}

	switch procedure {
	case applicationv1connect.ContentServicePutProcedure:
		put, ok := message.(*applicationv1.PutContentRequest)
		if !ok || put == nil || len(put.GetPayload()) == 0 {
			return ResourceTarget{}, ErrInvalidResourceTarget
		}
		if len(put.GetPayload()) > applicationv1.MaxUnaryPayloadBytes {
			return ResourceTarget{}, ErrPayloadTooLarge
		}
		return ResourceTarget{Kind: rule.ResourceKind}, nil
	case applicationv1connect.ContentServiceGetProcedure:
		get, ok := message.(*applicationv1.GetContentRequest)
		if !ok || get == nil || get.GetReference() == nil || get.GetReference().GetKind() != "blob" {
			return ResourceTarget{}, ErrInvalidResourceTarget
		}
		id, err := contentapi.AccessResourceID(get.GetReference().GetId())
		if err != nil {
			return ResourceTarget{}, ErrInvalidResourceTarget
		}
		return ResourceTarget{Kind: rule.ResourceKind, ID: id}, nil
	default:
		return ResourceTarget{}, ErrUnknownProcedure
	}
}

func hasUnknownFields(message protoreflect.Message) bool {
	if !message.IsValid() || len(message.GetUnknown()) != 0 {
		return true
	}
	unknown := false
	message.Range(func(field protoreflect.FieldDescriptor, value protoreflect.Value) bool {
		switch {
		case field.IsMap() && field.MapValue().Kind() == protoreflect.MessageKind:
			value.Map().Range(func(_ protoreflect.MapKey, entry protoreflect.Value) bool {
				unknown = hasUnknownFields(entry.Message())
				return !unknown
			})
		case field.IsList() && field.Kind() == protoreflect.MessageKind:
			list := value.List()
			for index := 0; index < list.Len() && !unknown; index++ {
				unknown = hasUnknownFields(list.Get(index).Message())
			}
		case field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind:
			unknown = hasUnknownFields(value.Message())
		}
		return !unknown
	})
	return unknown
}
