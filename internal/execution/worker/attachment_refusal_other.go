//go:build !linux && !darwin

package worker

import "time"

// There is no AF_UNIX peer-credential worker attachment on these platforms. The
// common real launch composition still consumes local admission and joins its
// refused Job, but this adapter admits no bytes and creates no attachment.
// Windows named-pipe impersonation is a different authentication model, not the
// per-frame SCM_CREDENTIALS binding this attachment requires, so it is not a
// weaker accepting path here (ADR-0129). The Unix-family transport lives in
// attachment_unix.go; its Linux peer-credential port authenticates every frame.
type Attachment struct{}

func (*Attachment) SetDeadline(time.Time) error { return errNativeUnavailable }
func (*Attachment) Read([]byte) (int, error)    { return 0, errNativeUnavailable }
func (*Attachment) Close() error                { return errNativeUnavailable }
