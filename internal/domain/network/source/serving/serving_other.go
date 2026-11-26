//go:build !linux && !darwin

package serving

import (
	"context"
	"errors"
	"io"

	"github.com/dianabuilds/ardents-network/internal/domain/network/source/servingdeclaration"
	"github.com/dianabuilds/ardents-network/internal/platform/processguard"
)

// errServingPlatformUnavailable is the explicit typed unavailable outcome for
// the Direct-Origin Source operation on platforms without the selected
// Unix-family input-custody and process-guard mechanism (ADR-0129). A missing
// required protection yields the same explicit refusal on every platform; it
// never supplies a weaker accepting path. The Unix-family operation lives in
// the _unix.go files; its native process guard refuses outside Linux through
// processguard, and Windows admits no original Source lifetime here.
var errServingPlatformUnavailable = errors.New("source serving platform unavailable")

// Outcome is the original attempt's process-independent terminal class.
type Outcome uint8

const (
	Refused Outcome = iota + 1
	Invalid
	Canceled
)

// Event is the bounded observation presented by the command.
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

// FailureDescription contains only fixed owner categories and bounded resource
// facts.
type FailureDescription struct {
	FirstOwner    string
	CleanupOwners string
	Resource      *processguard.FailureFacts
}

// DescribeFailure refuses to classify an original failure order on platforms
// without the Unix-family Source operation; no original lifetime runs here.
func DescribeFailure(operation, processClose error) FailureDescription {
	return FailureDescription{}
}

// ValidDeclaration refuses the detached operational input shape on platforms
// without the Unix-family Source operation; it admits no declaration.
func ValidDeclaration(plan servingdeclaration.ServingDeclaration) bool {
	return false
}

// WriteFailureReport refuses the bounded command report on platforms without
// the Unix-family Source operation; it opens no output.
func WriteFailureReport(out io.Writer, phase string, operation, processClose error) error {
	return errServingPlatformUnavailable
}

// Run refuses the original Direct-Origin Source operation on platforms without
// the selected Unix-family mechanism. It acquires no input, process guard or
// State and creates nothing; the caller observes the explicit Refused outcome.
func Run(ctx context.Context, planPath string, retained bool, report func(context.Context, Event) error) Outcome {
	return Refused
}

// AcquireInputs, ConfigureInputs and Supervise are intentionally absent here:
// their signatures borrow the Unix-family artifacts.ObservedInputs custody and
// the package-private sourceLifetime, which have no Windows equivalent. No
// portable consumer reaches them on this platform; Run is the typed refusal
// boundary (ADR-0129).
