package completion

// EncodeHistoryACK validates detached frame bytes and appends the closed
// history choice. It grants no original exchange or startup authority.
func EncodeHistoryACK(request [160]byte, history uint8) (reply [161]byte, err error) {
	if !canonicalFrame(request) || history > 1 {
		return reply, ErrInput
	}
	copy(reply[:160], request[:])
	reply[160] = history
	return reply, nil
}
