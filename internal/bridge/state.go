package bridge

const (
	memberActive   = "active"
	memberDraining = "draining"
	memberVerified = "verified"
	memberRetired  = "retired"
)

type durableState struct {
	Version    uint8           `json:"version"`
	Generation uint64          `json:"generation"`
	Previous   string          `json:"previous,omitempty"`
	Records    []memberRecord  `json:"records"`
	Regime     *regimeRecord   `json:"regime,omitempty"`
	Attempt    *attemptRecord  `json:"attempt,omitempty"`
	Contacts   []contactRecord `json:"contacts,omitempty"`
}

type regimeRecord struct {
	AttemptID      [32]byte `json:"attempt_id"`
	Trigger        string   `json:"trigger"`
	PolicyID       [32]byte `json:"policy_id"`
	Offset         uint64   `json:"offset_ns"`
	Manifest       [32]byte `json:"manifest"`
	Deadline       int64    `json:"deadline_unix_nano"`
	DeadlineOffset uint64   `json:"deadline_offset_ns"`
}

type attemptRecord struct {
	AttemptID      [32]byte `json:"attempt_id"`
	Started        uint64   `json:"started_offset_ns"`
	Deadline       int64    `json:"deadline_unix_nano"`
	DeadlineOffset uint64   `json:"deadline_offset_ns"`
	Terminal       string   `json:"terminal,omitempty"`
	TerminalOffset uint64   `json:"terminal_offset_ns,omitempty"`
}

type contactRecord struct {
	AttemptID [32]byte `json:"attempt_id"`
	InviteID  [32]byte `json:"invite_id"`
	ProfileID string   `json:"profile_id"`
	Slot      byte     `json:"slot"`
	Ordinal   byte     `json:"ordinal"`
	Started   uint64   `json:"started_offset_ns"`
	Terminal  uint64   `json:"terminal_offset_ns,omitempty"`
	Outcome   string   `json:"outcome,omitempty"`
	Cleanup   bool     `json:"cleanup"`
}

type memberRecord struct {
	InviteID   [32]byte `json:"invite_id"`
	Identity   [32]byte `json:"identity"`
	Family     [32]byte `json:"family"`
	Commitment [32]byte `json:"commitment,omitempty"`
	ProfileID  string   `json:"profile_id,omitempty"`
	Slot       byte     `json:"slot"`
	Generation byte     `json:"slot_generation"`
	Status     string   `json:"status"`
	Invite     []byte   `json:"invite,omitempty"`
}

func (state durableState) clone() durableState {
	cloned := state
	cloned.Records = make([]memberRecord, len(state.Records))
	copy(cloned.Records, state.Records)
	for index := range cloned.Records {
		cloned.Records[index].Invite = append([]byte(nil), state.Records[index].Invite...)
	}
	if state.Regime != nil {
		value := *state.Regime
		cloned.Regime = &value
	}
	if state.Attempt != nil {
		value := *state.Attempt
		cloned.Attempt = &value
	}
	cloned.Contacts = append([]contactRecord(nil), state.Contacts...)
	return cloned
}

func (state durableState) active(slot byte) (int, bool) {
	for index := range state.Records {
		if state.Records[index].Slot == slot && state.Records[index].Status == memberActive {
			return index, true
		}
	}
	return 0, false
}

func (state durableState) find(id [32]byte) (memberRecord, bool) {
	for _, record := range state.Records {
		if record.InviteID == id {
			return record, true
		}
	}
	return memberRecord{}, false
}

func (state *durableState) settleReplacements() {
	for slot := byte(0); slot < 2; slot++ {
		verified := -1
		for index := range state.Records {
			if state.Records[index].Slot == slot && state.Records[index].Status == memberVerified {
				verified = index
			}
		}
		if verified < 0 {
			continue
		}
		for index := range state.Records {
			if state.Records[index].Slot == slot && state.Records[index].Status == memberDraining {
				state.Records[index].Status = memberRetired
				state.Records[index].Invite = nil
				state.Records[index].Commitment = [32]byte{}
				state.Records[index].ProfileID = ""
			}
		}
		state.Records[verified].Status = memberActive
	}
}
