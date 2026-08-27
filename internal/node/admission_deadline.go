package node

import "time"

const maximumAdmissionTimeout = time.Minute

func validAdmissionTimeout(timeout time.Duration) bool {
	return timeout > 0 && timeout <= maximumAdmissionTimeout
}

func boundedAdmissionDeadline(now time.Time, timeout time.Duration, notAfter time.Time) time.Time {
	deadline := now.Add(timeout)
	if notAfter.Before(deadline) {
		return notAfter
	}
	return deadline
}
