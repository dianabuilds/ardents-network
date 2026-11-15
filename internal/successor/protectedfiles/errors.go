package protectedfiles

import "errors"

// ErrInput preserves the existing bounded input refusal identity.
var ErrInput = errors.New("installation: invalid generation authentication input")

// ErrNativeUnavailable refuses unavailable or foreign native custody.
var ErrNativeUnavailable = errors.New("installation: native request custody unavailable")

// ErrChanged refuses a different original file after admission.
var ErrChanged = errors.New("installation request: original input changed")
