package serving

// Outcome is the original attempt's process-independent terminal class.
type Outcome uint8

const (
	Refused Outcome = iota + 1
	Invalid
	Canceled
)

// Event is the bounded observation presented by the command. The causes
// retain physical failures for State and diagnostic classification.
type Event struct {
	Phase, Outcome        string
	Failure, ProcessClose error
}

// Terminal keeps the original failure order through physical join.
type Terminal struct {
	FirstOwner                                   string
	First, Process, Wait, StateClose, InputClose error
}

func (*Terminal) Error() string { return "original Source failed" }

func (result *Terminal) Unwrap() []error {
	var causes []error
	for _, cause := range []error{result.First, result.Process, result.Wait, result.StateClose, result.InputClose} {
		if cause != nil {
			causes = append(causes, cause)
		}
	}
	return causes
}
