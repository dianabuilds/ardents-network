// Package forwarding owns the closed Route forwarding listener, receiving
// spend root, Carrier sessions and child links, and joined shutdown. It
// rechecks current State for each admission and selected next hop. Node
// supplies the shared Hosting lease and class-2 reservation policy.
package forwarding
