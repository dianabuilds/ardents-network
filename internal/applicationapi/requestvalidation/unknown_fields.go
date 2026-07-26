// Package requestvalidation owns structural protobuf validation shared by
// Application adapters. It does not own product request semantics, admission,
// or generated protocol types.
package requestvalidation

import (
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func HasUnknownFields(message proto.Message) bool {
	return message == nil || hasUnknownFields(message.ProtoReflect())
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
