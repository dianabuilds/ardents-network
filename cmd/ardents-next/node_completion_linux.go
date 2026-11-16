package main

import "time"

type nodeCompletion struct {
	outcome        string
	stateError     error
	first, cleanup string
}

// A timeout reports unavailability but cannot make the original Node or State
// appear joined. The final result is retained until physical completion.
func awaitNodeCompletion(joined <-chan nodeCompletion, timeout <-chan time.Time, notify func()) (nodeCompletion, bool) {
	select {
	case result := <-joined:
		return result, false
	case <-timeout:
		notify()
		return <-joined, true
	}
}
