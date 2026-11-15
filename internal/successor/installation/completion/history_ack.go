package completion

// encodeHistoryACK validates detached frame bytes and appends the closed
// history choice. It grants no original exchange or startup authority.
func encodeHistoryACK(request [160]byte, history uint8) (reply [161]byte, err error) {
	if !canonicalFrame(request) || history > 1 {
		return reply, ErrInput
	}
	copy(reply[:160], request[:])
	reply[160] = history
	return reply, nil
}

// EncodeJoinACK encodes a detached fixed-role history choice.
func EncodeJoinACK(request [160]byte, history uint8) ([161]byte, error) {
	return encodeHistoryACK(request, history)
}

// EncodeDescriptorACK encodes a detached fixed-role history choice.
func EncodeDescriptorACK(request [160]byte, history uint8) ([161]byte, error) {
	return encodeHistoryACK(request, history)
}

// EncodeForwardingACK encodes a detached fixed-role history choice.
func EncodeForwardingACK(request [160]byte, history uint8) ([161]byte, error) {
	return encodeHistoryACK(request, history)
}
