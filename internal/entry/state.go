package entry

const (
	memberActive  = "active"
	memberRetired = "retired"
)

type durableState struct {
	Version    uint8          `json:"version"`
	Generation uint64         `json:"generation"`
	Previous   string         `json:"previous,omitempty"`
	Records    []memberRecord `json:"records"`
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

func retireMember(record *memberRecord) {
	record.Status = memberRetired
	record.Invite = nil
}
