// Package durable owns an independent exclusive Publication root, its canonical
// generation floor and retained public record layout. Reopening public evidence
// never restores an accepting Publisher or private Instance authority.
// Generation.CheckCommit only compares an immutable local restriction after
// fresh native Check; it never waits for the storage mutex. Reservation/root
// retirement seals that comparison before joining original native effects.
// The exclusive root lease and directory flush are native on the Unix family
// (Linux and macOS) with an advisory flock and on Windows through handle-based
// identity checks; other hosts refuse before any root state.
package durable
