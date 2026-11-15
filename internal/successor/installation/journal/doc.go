// Package journal decodes bounded Node and Source invocation and retirement
// records. Detached records provide no native custody, Release permission,
// process completion or authority to start or recover a runtime.
package journal

import "errors"

var (
	ErrInput       = errors.New("invalid Installation preparation journal")
	ErrBinding     = errors.New("installation preparation journal differs")
	ErrUnavailable = errors.New("installation preparation journal unavailable")
)
