package network

import (
	"errors"
	"math"
	"time"
)

const (
	attemptUnused uint8 = iota
	attemptInFlight
	attemptCompleted
	attemptFailed
)

const (
	acquisitionValid       = uint8(1)
	acquisitionInterrupted = uint8(9)
	acquisitionLastOutcome = uint8(13)
	acquisitionRefresh     = uint8(1)
	acquisitionDuration    = 15 * time.Second
)

var ErrAcquisitionBackoff = errors.New("acquisition is in durable backoff")

// AcquisitionFacts is a copied persistence projection, not permission to
// contact a Source. The four attempt slots are two Latest and two exact-digest
// selectors. Their retained status/outcome numbers cannot be renumbered.
type AcquisitionFacts struct {
	Cycle              uint64
	Active             bool
	Purpose            uint8
	Started, Deadline  int64
	Seed               [32]byte
	Order              [2]uint8
	Attempts, Outcomes [4]uint8
	RequestedDigests   [2][32]byte
	ObservedEpochs     [4]uint64
	ObservedDigests    [4][32]byte
	Exposures          [][32]byte
	Failures           uint64
	Backoff            uint8
	NextAttempt        int64
}

// Acquisition is immutable finite contact history inside the State aggregate's
// commit boundary. Transport, randomness, cancellation and local live exposure
// guards belong to the application. Proposed transitions authorize no I/O.
type Acquisition struct{ facts AcquisitionFacts }

func RestoreAcquisition(facts AcquisitionFacts) (Acquisition, error) {
	if len(facts.Exposures) > 2 || facts.Backoff > 5 || facts.Purpose > acquisitionRefresh ||
		(facts.Active && (facts.Purpose != acquisitionRefresh || facts.Started <= 0 || facts.Deadline <= facts.Started)) ||
		(facts.Cycle > 0 && (facts.Order[0] > 1 || facts.Order[1] > 1 || facts.Order[0] == facts.Order[1])) {
		return Acquisition{}, errors.New("acquisition history is invalid")
	}
	for slot, status := range facts.Attempts {
		if status > attemptFailed || facts.Outcomes[slot] > acquisitionLastOutcome ||
			(facts.ObservedEpochs[slot] == 0) != (facts.ObservedDigests[slot] == [32]byte{}) {
			return Acquisition{}, errors.New("acquisition attempt evidence is invalid")
		}
	}
	for source, digest := range facts.RequestedDigests {
		if (digest == [32]byte{}) != (facts.Attempts[source+2] == attemptUnused) {
			return Acquisition{}, errors.New("acquisition digest attempt has no exact selector")
		}
	}
	facts.Exposures = append([][32]byte(nil), facts.Exposures...)
	return Acquisition{facts: facts}, nil
}

func (acquisition Acquisition) Facts() AcquisitionFacts {
	facts := acquisition.facts
	facts.Exposures = append([][32]byte(nil), facts.Exposures...)
	return facts
}

func hasExposure(exposures [][32]byte, identity [32]byte) bool {
	for _, retained := range exposures {
		if retained == identity {
			return true
		}
	}
	return false
}

// CheckExposures refuses growth before a changed plan's first contact.
func (acquisition Acquisition) CheckExposures(proposed [2][32]byte) error {
	exposures := acquisition.Facts().Exposures
	for _, identity := range proposed {
		if !hasExposure(exposures, identity) {
			exposures = append(exposures, identity)
		}
	}
	if len(exposures) > 2 {
		return errors.New("direct Source exposure history is full")
	}
	return nil
}

// Start resumes the original deadline/order or proposes a new fifteen-second
// cycle. Expired histories are terminalized, never renewed. Changed tells the
// owner whether a durable commit is required before exposing the result.
func (acquisition Acquisition) Start(now time.Time, seed [32]byte) (next Acquisition, changed, expired bool, err error) {
	facts := acquisition.Facts()
	if !acquisitionTimeValid(now) || (facts.Active && now.Unix() < facts.Started) {
		return Acquisition{}, false, false, errors.New("acquisition time is invalid")
	}
	if facts.Active {
		if now.Unix() >= facts.Deadline {
			for slot := range facts.Attempts {
				interruptAttempt(&facts, slot)
			}
			facts.Active = false
			applyAcquisitionBackoff(&facts, now)
			return Acquisition{facts}, true, true, nil
		}
		for slot := 2; slot < len(facts.Attempts); slot++ {
			changed = interruptAttempt(&facts, slot) || changed
		}
		return Acquisition{facts}, changed, false, nil
	}
	if facts.NextAttempt > now.Unix() {
		return Acquisition{}, false, false, ErrAcquisitionBackoff
	}
	if facts.Cycle == math.MaxUint64 {
		return Acquisition{}, false, false, errors.New("acquisition cycle cannot advance")
	}
	facts.Cycle++
	facts.Active = true
	facts.Purpose = acquisitionRefresh
	facts.Started, facts.Deadline = now.Unix(), now.Add(acquisitionDuration).Unix()
	facts.Attempts, facts.Outcomes = [4]uint8{}, [4]uint8{}
	facts.RequestedDigests = [2][32]byte{}
	facts.ObservedEpochs, facts.ObservedDigests = [4]uint64{}, [4][32]byte{}
	facts.Seed, facts.Order = seed, [2]uint8{0, 1}
	if seed[0]&1 != 0 {
		facts.Order = [2]uint8{1, 0}
	}
	return Acquisition{facts}, true, false, nil
}

// LatestDisposition distinguishes a fresh proposed contact from a consumed
// selector. A crash with an unrecorded result stays consumed after reopening.
type LatestDisposition struct {
	Contact, Changed bool
	Outcome          uint8
}

func (acquisition Acquisition) BeginLatest(source int, exposure [32]byte) (Acquisition, LatestDisposition, error) {
	facts := acquisition.Facts()
	if !facts.Active || source < 0 || source > 1 {
		return Acquisition{}, LatestDisposition{}, errors.New("LATEST source attempt is outside the active cycle")
	}
	if interruptAttempt(&facts, source) {
		return Acquisition{facts}, LatestDisposition{Changed: true, Outcome: acquisitionInterrupted}, nil
	}
	if facts.Attempts[source] != attemptUnused {
		return acquisition, LatestDisposition{Outcome: facts.Outcomes[source]}, nil
	}
	if err := retainExposure(&facts, exposure); err != nil {
		return Acquisition{}, LatestDisposition{}, err
	}
	facts.Attempts[source] = attemptInFlight
	return Acquisition{facts}, LatestDisposition{Contact: true, Changed: true}, nil
}

func (acquisition Acquisition) BeginDigest(source int, digest, exposure [32]byte) (Acquisition, error) {
	facts := acquisition.Facts()
	if !facts.Active || source < 0 || source > 1 || digest == [32]byte{} || facts.Attempts[source+2] != attemptUnused {
		return Acquisition{}, errors.New("by-digest source attempt is not available")
	}
	if err := retainExposure(&facts, exposure); err != nil {
		return Acquisition{}, err
	}
	facts.Attempts[source+2], facts.RequestedDigests[source] = attemptInFlight, digest
	return Acquisition{facts}, nil
}

func (acquisition Acquisition) CompleteDigest(source int, responseCompleted bool) (Acquisition, error) {
	facts := acquisition.Facts()
	if !facts.Active || source < 0 || source > 1 || facts.Attempts[source+2] != attemptInFlight {
		return Acquisition{}, errors.New("by-digest source attempt is not started")
	}
	facts.Attempts[source+2] = attemptFailed
	if responseCompleted {
		facts.Attempts[source+2] = attemptCompleted
	}
	return Acquisition{facts}, nil
}

func retainExposure(facts *AcquisitionFacts, exposure [32]byte) error {
	if !hasExposure(facts.Exposures, exposure) {
		if len(facts.Exposures) == 2 {
			return errors.New("direct Source exposure history is full")
		}
		facts.Exposures = append(facts.Exposures, exposure)
	}
	return nil
}

func interruptAttempt(facts *AcquisitionFacts, slot int) bool {
	status := facts.Attempts[slot]
	if status != attemptInFlight && (status != attemptCompleted || facts.Outcomes[slot] != 0) {
		return false
	}
	facts.Attempts[slot], facts.Outcomes[slot] = attemptFailed, acquisitionInterrupted
	return true
}

// Finish preserves consumed attempts, records terminal outcomes and updates
// bounded durable backoff. It does not select or accept any observed Epoch.
func (acquisition Acquisition) Finish(now time.Time, outcomes [4]uint8) (Acquisition, error) {
	facts := acquisition.Facts()
	if !acquisitionTimeValid(now) || now.Unix() < facts.Started {
		return Acquisition{}, errors.New("acquisition completion time is invalid")
	}
	facts.Active = false
	for slot, outcome := range outcomes {
		if outcome != 0 {
			facts.Outcomes[slot] = outcome
		}
	}
	for slot, status := range facts.Attempts {
		if facts.Outcomes[slot] == 0 && status != attemptUnused {
			facts.Outcomes[slot] = acquisitionInterrupted
		}
	}
	for slot, outcome := range facts.Outcomes {
		if outcome == acquisitionValid {
			facts.Attempts[slot] = attemptCompleted
		} else if outcome != 0 {
			facts.Attempts[slot] = attemptFailed
		}
	}
	anyValid := false
	for _, outcome := range facts.Outcomes {
		if outcome != 0 && outcome != acquisitionValid {
			applyAcquisitionBackoff(&facts, now)
			return RestoreAcquisition(facts)
		}
		anyValid = anyValid || outcome == acquisitionValid
	}
	if !anyValid {
		applyAcquisitionBackoff(&facts, now)
		return RestoreAcquisition(facts)
	}
	facts.Failures, facts.Backoff, facts.NextAttempt = 0, 0, 0
	return RestoreAcquisition(facts)
}

func acquisitionTimeValid(now time.Time) bool {
	return !now.IsZero() && now.Unix() > 0 && now.Unix() <= math.MaxInt64-1800
}

func applyAcquisitionBackoff(facts *AcquisitionFacts, now time.Time) {
	if facts.Failures < math.MaxInt64 {
		facts.Failures++
	}
	level := min(facts.Failures-1, 5)
	facts.Backoff = uint8(level)
	bases := [...]int64{60, 120, 240, 480, 960, 1800}
	base := bases[level]
	facts.NextAttempt = now.Unix() + base/2 + int64(facts.Seed[1])*(base/2+1)/256
}
