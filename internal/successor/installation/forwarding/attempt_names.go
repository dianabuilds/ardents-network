package forwarding

import (
	"fmt"
	posixpath "path"
)

// The closed initial-generation profile retains every attempt, never recycling
// a pathname or deleting history on exhaustion. This is an Installation read
// bound, not another network retry, token or Hosting allowance.
const attemptLimit = 64
const historyDirectoryLimit = 2*attemptLimit + 16

type attemptNumber uint32

func (n attemptNumber) directory(root string) string {
	if n == 0 {
		return posixpath.Join(root, "forwarding-startup")
	}
	return posixpath.Join(root, fmt.Sprintf("forwarding-invocation-%08d", n))
}

func (n attemptNumber) journal(root string) string {
	if n == 0 {
		return root + ".forwarding-first-start-journal"
	}
	return root + fmt.Sprintf(".forwarding-invocation-%08d-journal", n)
}

func (n attemptNumber) retirement(root string) string {
	return posixpath.Join(root, fmt.Sprintf("forwarding-retirement-%08d", n))
}

func (n attemptNumber) retirementJournal(root string) string {
	return root + fmt.Sprintf(".forwarding-retirement-%08d-journal", n)
}
