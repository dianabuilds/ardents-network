package entry

const (
	memberActive   = "active"
	memberDraining = "draining"
	memberVerified = "verified"
	memberRetired  = "retired"
)

type durableState struct {
	Version    uint8             `json:"version"`
	Generation uint64            `json:"generation"`
	Previous   string            `json:"previous,omitempty"`
	Records    []memberRecord    `json:"records"`
	Attempt    *attemptRecord    `json:"attempt,omitempty"`
	Contacts   []contactRecord   `json:"contacts,omitempty"`
	Admissions []admissionRecord `json:"admissions,omitempty"`
}

type attemptRecord struct {
	ID       [32]byte `json:"id"`
	Started  int64    `json:"started_unix_nano"`
	Deadline int64    `json:"deadline_unix_nano"`
	Terminal string   `json:"terminal,omitempty"`
	Ended    int64    `json:"ended_unix_nano,omitempty"`
}

type contactRecord struct {
	AttemptID [32]byte `json:"attempt_id"`
	InviteID  [32]byte `json:"invite_id"`
	Slot      byte     `json:"slot"`
	Ordinal   byte     `json:"ordinal"`
	Started   int64    `json:"started_unix_nano"`
	Terminal  int64    `json:"terminal_unix_nano,omitempty"`
	Outcome   string   `json:"outcome,omitempty"`
	Cleanup   bool     `json:"cleanup"`
}

type memberRecord struct {
	InviteID   [32]byte `json:"invite_id"`
	Identity   [32]byte `json:"identity"`
	Family     [32]byte `json:"family"`
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
	if state.Attempt != nil {
		value := *state.Attempt
		cloned.Attempt = &value
	}
	cloned.Contacts = append([]contactRecord(nil), state.Contacts...)
	cloned.Admissions = append([]admissionRecord(nil), state.Admissions...)
	return cloned
}

func (state *durableState) settleReplacements() bool {
	changed := false
	for slot := byte(0); slot < 2; slot++ {
		verified := -1
		for index := range state.Records {
			if state.Records[index].Slot == slot && state.Records[index].Status == memberVerified {
				verified = index
			}
		}
		for index := range state.Records {
			if state.Records[index].Slot == slot && state.Records[index].Status == memberDraining {
				retireMember(&state.Records[index])
				changed = true
			}
		}
		if verified >= 0 {
			state.Records[verified].Status = memberActive
			changed = true
		}
	}
	return changed
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

func retireMember(record *memberRecord) {
	record.Status = memberRetired
	record.Invite = nil
}
