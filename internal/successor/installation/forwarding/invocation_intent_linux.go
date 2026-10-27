package forwarding

import "encoding/json"

// Subsequent invocation bytes have their own closed grammar. First-start bytes
// remain unchanged; a copied first intent cannot omit predecessor retirement.
type invocationIntent struct {
	Schema               string `json:"schema"`
	BindingSHA256        string `json:"binding_sha256"`
	InitialHistorySHA256 string `json:"initial_history_sha256"`
	TargetsSHA256        string `json:"targets_sha256"`
	UID                  uint32 `json:"uid"`
	GID                  uint32 `json:"gid"`
	Attempt              uint32 `json:"attempt"`
	PredecessorSHA256    string `json:"predecessor_sha256"`
}

func encodeStartIntent(b binding, bindingSHA, initialSHA, targetsSHA string, number attemptNumber, predecessor string) ([]byte, error) {
	if number >= attemptLimit || !validStartDigest(bindingSHA) || !validStartDigest(initialSHA) || !validStartDigest(targetsSHA) {
		return nil, ErrBinding
	}
	var value any
	if number == 0 {
		if predecessor != "" {
			return nil, ErrBinding
		}
		value = startIntent{"ardents-forwarding-start-intent-v1", bindingSHA, initialSHA, targetsSHA, b.UID, b.GID, 0}
	} else {
		if !validStartDigest(predecessor) {
			return nil, ErrBinding
		}
		value = invocationIntent{"ardents-forwarding-invocation-intent-v1", bindingSHA, initialSHA, targetsSHA, b.UID, b.GID, uint32(number), predecessor}
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	return append(raw, '\n'), nil
}

func checkStartIntent(raw []byte, b binding, bindingSHA, initialSHA string, number attemptNumber, predecessor string) error {
	if number >= attemptLimit {
		return ErrBinding
	}
	var targets string
	if number == 0 {
		var intent startIntent
		if predecessor != "" || !canonicalStartRecord(raw, &intent) || intent.Schema != "ardents-forwarding-start-intent-v1" || intent.BindingSHA256 != bindingSHA || intent.InitialHistorySHA256 != initialSHA || intent.UID != b.UID || intent.GID != b.GID || intent.Attempt != 0 {
			return ErrBinding
		}
		targets = intent.TargetsSHA256
	} else {
		var intent invocationIntent
		if !validStartDigest(predecessor) || !canonicalStartRecord(raw, &intent) || intent.Schema != "ardents-forwarding-invocation-intent-v1" || intent.BindingSHA256 != bindingSHA || intent.InitialHistorySHA256 != initialSHA || intent.UID != b.UID || intent.GID != b.GID || intent.Attempt != uint32(number) || intent.PredecessorSHA256 != predecessor {
			return ErrBinding
		}
		targets = intent.TargetsSHA256
	}
	if !validStartDigest(targets) {
		return ErrBinding
	}
	return nil
}
