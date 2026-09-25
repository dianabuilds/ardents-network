package resource

import "errors"

// ErrOwnerResidentProcessGone marks a process that left an owner cgroup while
// its resident memory was being sampled. Exhausted retries remain a failure.
var ErrOwnerResidentProcessGone = errors.New("owner process disappeared during resident sample")
