package systemd

import (
	"encoding/binary"
	"unicode/utf8"
)

// Validate every nested length before the library decoder sees a bounded
// property reply. Neither outer message size nor a valid signature alone
// prevents a malicious array/string length from requesting a large allocation.
type managerValueType struct {
	kind     byte
	children []managerValueType
}

func managerValueTypes(signature string, depth int) ([]managerValueType, error) {
	if depth > 16 || len(signature) == 0 || len(signature) > 255 {
		return nil, ErrObservation
	}
	var values []managerValueType
	for position := 0; position < len(signature); {
		value, next, err := managerValueTypeAt(signature, position, depth)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
		position = next
	}
	return values, nil
}

func managerValueTypeAt(signature string, position, depth int) (managerValueType, int, error) {
	if depth > 16 || position >= len(signature) {
		return managerValueType{}, position, ErrObservation
	}
	value := managerValueType{kind: signature[position]}
	position++
	switch value.kind {
	case 'y', 'b', 'n', 'q', 'i', 'u', 'x', 't', 'd', 's', 'o', 'g', 'v':
		return value, position, nil
	case 'a':
		child, next, err := managerValueTypeAt(signature, position, depth+1)
		if err != nil {
			return value, position, err
		}
		value.children = []managerValueType{child}
		return value, next, nil
	case '(', '{':
		if value.kind == '{' && (position < 2 || signature[position-2] != 'a') {
			return value, position, ErrObservation
		}
		end := byte(')')
		if value.kind == '{' {
			end = '}'
		}
		for position < len(signature) && signature[position] != end {
			child, next, err := managerValueTypeAt(signature, position, depth+1)
			if err != nil {
				return value, position, err
			}
			value.children = append(value.children, child)
			position = next
		}
		if position >= len(signature) || len(value.children) == 0 || value.kind == '{' && (len(value.children) != 2 || !managerBasicType(value.children[0].kind)) {
			return value, position, ErrObservation
		}
		return value, position + 1, nil
	default:
		// File descriptors are never negotiated or accepted by this transport.
		return value, position, ErrObservation
	}
}

func managerBasicType(kind byte) bool {
	switch kind {
	case 'y', 'b', 'n', 'q', 'i', 'u', 'x', 't', 'd', 's', 'o', 'g':
		return true
	}
	return false
}

func managerValueAlignment(kind byte) int {
	switch kind {
	case 'n', 'q':
		return 2
	case 'b', 'i', 'u', 's', 'o', 'a':
		return 4
	case 'x', 't', 'd', '(', '{':
		return 8
	default:
		return 1
	}
}

func managerAlign(frame []byte, position, end, alignment int) (int, error) {
	next := (position + alignment - 1) &^ (alignment - 1)
	if position > end || next > end {
		return position, ErrObservation
	}
	for _, padding := range frame[position:next] {
		if padding != 0 {
			return position, ErrObservation
		}
	}
	return next, nil
}

func verifyManagerValues(frame []byte, position int, signature string, order binary.ByteOrder) error {
	values, err := managerValueTypes(signature, 0)
	if err != nil {
		return err
	}
	for _, value := range values {
		position, err = managerValueEnd(frame, position, len(frame), value, order, 0)
		if err != nil {
			return err
		}
	}
	if position != len(frame) {
		return ErrObservation
	}
	return nil
}

func managerValueEnd(frame []byte, position, end int, value managerValueType, order binary.ByteOrder, depth int) (int, error) {
	if depth > 16 || end > len(frame) {
		return position, ErrObservation
	}
	position, err := managerAlign(frame, position, end, managerValueAlignment(value.kind))
	if err != nil {
		return position, err
	}
	size := 0
	switch value.kind {
	case 'y':
		size = 1
	case 'n', 'q':
		size = 2
	case 'b', 'i', 'u':
		size = 4
	case 'x', 't', 'd':
		size = 8
	case 's', 'o', 'g':
		text, next, err := referenceScalar(frame, position, end, value.kind, order)
		if err != nil || !utf8.ValidString(text) {
			return position, ErrObservation
		}
		if value.kind == 'g' && text != "" {
			if _, err := managerValueTypes(text, depth+1); err != nil {
				return position, err
			}
		}
		return next, nil
	case 'v':
		signature, next, err := referenceScalar(frame, position, end, 'g', order)
		if err != nil {
			return position, err
		}
		children, err := managerValueTypes(signature, depth+1)
		if err != nil || len(children) != 1 {
			return position, ErrObservation
		}
		return managerValueEnd(frame, next, end, children[0], order, depth+1)
	case 'a':
		if position+4 > end || len(value.children) != 1 {
			return position, ErrObservation
		}
		length := uint64(order.Uint32(frame[position : position+4]))
		position, err = managerAlign(frame, position+4, end, managerValueAlignment(value.children[0].kind))
		if err != nil || length > uint64(end-position) {
			return position, ErrObservation
		}
		limit := position + int(length)
		seen := make(map[string]struct{})
		for position < limit {
			previous := position
			child := value.children[0]
			if child.kind == '{' {
				start, err := managerAlign(frame, position, limit, 8)
				if err != nil {
					return position, err
				}
				keyEnd, err := managerValueEnd(frame, start, limit, child.children[0], order, depth+1)
				if err != nil {
					return position, err
				}
				key := string(frame[start:keyEnd])
				if _, duplicate := seen[key]; duplicate {
					return position, ErrObservation
				}
				seen[key] = struct{}{}
			}
			position, err = managerValueEnd(frame, position, limit, child, order, depth+1)
			if err != nil || position <= previous {
				return previous, ErrObservation
			}
		}
		return position, nil
	case '(', '{':
		for _, child := range value.children {
			position, err = managerValueEnd(frame, position, end, child, order, depth+1)
			if err != nil {
				return position, err
			}
		}
		return position, nil
	default:
		return position, ErrObservation
	}
	if position+size > end || value.kind == 'b' && order.Uint32(frame[position:position+4]) > 1 {
		return position, ErrObservation
	}
	return position + size, nil
}
