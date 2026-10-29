package forwarding

import "encoding/json"

type historyStartIntent struct {
	startIntent
	History uint8 `json:"history"`
}

type historyInvocationIntent struct {
	invocationIntent
	History uint8 `json:"history"`
}

func encodeHistoryStartIntent(b binding, bindingSHA, initialSHA, targetsSHA string, number attemptNumber, predecessor string, history uint8) ([]byte, error) {
	if history > 1 {
		return nil, ErrBinding
	}
	legacy, err := encodeStartIntent(b, bindingSHA, initialSHA, targetsSHA, number, predecessor)
	if err != nil {
		return nil, err
	}
	var value any
	if number == 0 {
		var base startIntent
		if json.Unmarshal(legacy, &base) != nil {
			return nil, ErrBinding
		}
		base.Schema = "ardents-forwarding-start-intent-v2"
		value = historyStartIntent{base, history}
	} else {
		var base invocationIntent
		if json.Unmarshal(legacy, &base) != nil {
			return nil, ErrBinding
		}
		base.Schema = "ardents-forwarding-invocation-intent-v2"
		value = historyInvocationIntent{base, history}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

// Legacy completed transcripts keep their original first/subsequent meaning.
// New intents bind an explicit independently computed choice; this parser's
// copied byte never proves the chain or allows a live exchange to use legacy ACK.
func readStartHistoryIntent(raw []byte, b binding, bindingSHA, initialSHA string, number attemptNumber, predecessor string) (history uint8, extended bool, err error) {
	var header struct{ Schema string }
	if json.Unmarshal(raw, &header) != nil {
		return 0, false, ErrBinding
	}
	var legacy any
	switch header.Schema {
	case "ardents-forwarding-start-intent-v2":
		var intent historyStartIntent
		if number != 0 || !canonicalStartRecord(raw, &intent) || intent.History > 1 {
			return 0, false, ErrBinding
		}
		history, extended = intent.History, true
		intent.Schema = "ardents-forwarding-start-intent-v1"
		legacy = intent.startIntent
	case "ardents-forwarding-invocation-intent-v2":
		var intent historyInvocationIntent
		if number == 0 || !canonicalStartRecord(raw, &intent) || intent.History > 1 {
			return 0, false, ErrBinding
		}
		history, extended = intent.History, true
		intent.Schema = "ardents-forwarding-invocation-intent-v1"
		legacy = intent.invocationIntent
	case "ardents-forwarding-start-intent-v1", "ardents-forwarding-invocation-intent-v1":
		if number != 0 {
			history = 1
		}
		return history, false, checkLegacyStartIntent(raw, b, bindingSHA, initialSHA, number, predecessor)
	default:
		return 0, false, ErrBinding
	}
	normalized, err := json.Marshal(legacy)
	if err != nil {
		return 0, false, err
	}
	if err := checkLegacyStartIntent(append(normalized, '\n'), b, bindingSHA, initialSHA, number, predecessor); err != nil {
		return 0, false, err
	}
	return history, extended, nil
}
