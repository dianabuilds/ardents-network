package duty

import (
	"time"
)

// NodePhase describes execution reported by the local Node owner. It does not
// grant an Epoch assignment or make an otherwise expired assignment current.
type NodePhase string

const (
	NodePrepared    NodePhase = "prepared"
	NodeQuarantined NodePhase = "quarantined"
	NodeLive        NodePhase = "live"
)

// NodeParticipation is the local restriction implied by an authenticated
// assignment. Epoch and record bounds are separate inputs: neither can extend
// the other. The producer remains responsible for releasing joined work.
type NodeParticipation struct {
	Identity                [32]byte
	Family                  string
	Assignment              string
	EpochUntil, RecordUntil time.Time
}

// RetainNode owns classification and validity of the local exclusion. Node
// reports its phase; it cannot choose an ordinary-Initiator exemption or turn
// its assignment into a Direct Source exposure record.
func (store *store) RetainNode(producer [32]byte, participation NodeParticipation, phase NodePhase) error {
	restriction, err := nodeParticipationRestriction(participation, phase)
	if err != nil {
		return err
	}
	return store.Replace(producer, []Duty{restriction})
}
