package capability

import "fmt"

const (
	CodeMissing         = "privacy.channel_grant.missing"
	CodeNotYetValid     = "privacy.channel_grant.not_yet_valid"
	CodeExpired         = "privacy.channel_grant.expired"
	CodeRevoked         = "privacy.channel_grant.revoked"
	CodeScopeDenied     = "privacy.channel_grant.scope_denied"
	CodeIssuerUntrusted = "privacy.channel_grant.issuer_untrusted"
	CodeInvalid         = "privacy.channel_grant.invalid"
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
