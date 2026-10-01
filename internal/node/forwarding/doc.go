// Package forwarding owns the closed Route forwarding duty: its listener,
// receiving spend root, retained Carrier sessions, child links and joined
// shutdown. Session readers can outlive an accepted producer, so the listener
// joins producers before the session set joins readers and releases the root.
// Node supplies the shared Hosting lease and class-2 reservation policy.
package forwarding
