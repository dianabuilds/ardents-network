package main

import (
	"bytes"
	"encoding/json"
	"time"
)

// decodeNodePlan is the shared canonical plan codec: a role plan is admitted
// only as its exact original canonical JSON bytes, then prepared through the
// role's own validator. The plan type, maximum size and prepare hook stay
// role-bound; this skeleton holds no schema, key or authority.
func decodeNodePlan[P any](raw []byte, now time.Time, maximum int, prepare func(P, time.Time) ([]byte, error)) (P, []byte, error) {
	var p P
	if len(raw) == 0 || len(raw) > maximum || json.Unmarshal(raw, &p) != nil {
		return p, nil, errSourceControlInput
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return p, nil, errSourceControlInput
	}
	prepared, err := prepare(p, now)
	return p, prepared, err
}

// decodeNodeProfilePlan admits a role profile plan whose preparation is a
// detached verified fact bundle rather than a single signing input.
func decodeNodeProfilePlan[P any, Q any](raw []byte, now time.Time, maximum int, prepare func(P, time.Time) (Q, error)) (P, Q, error) {
	var p P
	var prepared Q
	if len(raw) == 0 || len(raw) > maximum || json.Unmarshal(raw, &p) != nil {
		return p, prepared, errSourceControlInput
	}
	canonical, err := json.Marshal(p)
	if err != nil || !bytes.Equal(raw, append(canonical, '\n')) {
		return p, prepared, errSourceControlInput
	}
	prepared, err = prepare(p, now)
	return p, prepared, err
}
