// Package preparation composes initial Source State and participation. On
// Linux it retains the original inputs and root directories through signed
// Epoch acceptance and initializes empty participation only afterwards. State
// remains the owner of authenticated history and its durable lifecycle. Source
// command and inspection refuse their Linux-only operations on other hosts.
package preparation
