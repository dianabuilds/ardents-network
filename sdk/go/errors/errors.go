// Package errors defines stable Application Interface failures.
// It does not own transport error encoding or server-side policy.
package errors

import "fmt"

type Code string

const (
	InvalidArgument   Code = "invalid_argument"
	Unauthenticated   Code = "unauthenticated"
	Forbidden         Code = "forbidden"
	NotFound          Code = "not_found"
	Conflict          Code = "conflict"
	Unavailable       Code = "unavailable"
	IntegrityFailed   Code = "integrity_failed"
	ResourceExhausted Code = "resource_exhausted"
	Internal          Code = "internal"
)

type Error struct {
	Code      Code
	Operation string
	Message   string
	Retryable bool
	Details   map[string]string
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Operation == "" {
		return e.Message
	}
	return fmt.Sprintf("%s: %s", e.Operation, e.Message)
}
