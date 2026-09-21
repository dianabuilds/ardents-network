// Package node owns one authenticated Node duty from admission through terminal
// cleanup. A closed forwarding server owns accepted producers, pool interruption,
// and duty roots; its session set separately owns outgoing Carrier readers,
// joins them after all producers, and retains their terminal cleanup result.
package node
