package capability

import "fmt"

const (
	CodeMissing         = "privacy.capability.missing"
	CodeNotYetValid     = "privacy.capability.not_yet_valid"
	CodeExpired         = "privacy.capability.expired"
	CodeRevoked         = "privacy.capability.revoked"
	CodeScopeDenied     = "privacy.capability.scope_denied"
	CodeIssuerUntrusted = "privacy.capability.issuer_untrusted"
	CodeInvalid         = "privacy.capability.invalid"
)

type Error struct {
	Code string
	err  error
}

func (e *Error) Error() string {
	if e == nil {
		return "capability operation failed"
	}
	return e.Code + ": capability operation failed"
}

func (e *Error) Unwrap() error { return e.err }

func (e *Error) FailureCode() string { return e.Code }

func capabilityError(code, detail string) error {
	return &Error{Code: code, err: fmt.Errorf("%s", detail)}
}
